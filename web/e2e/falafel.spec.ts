import { Buffer } from 'node:buffer'
import { readFile } from 'node:fs/promises'

import { expect, test, type Page } from '@playwright/test'

function payload(size = 1024 * 1024 + 93): Buffer {
  const buffer = Buffer.alloc(size)
  for (let i = 0; i < size; i += 1) buffer[i] = i % 251
  return buffer
}

async function selectFile(page: Page, name: string, buffer: Buffer): Promise<void> {
  await page.locator('input[type="file"]').setInputFiles({
    name,
    mimeType: 'application/octet-stream',
    buffer,
  })
  await expect(page.getByText(name, { exact: true })).toBeVisible()
}

async function startSending(page: Page): Promise<string> {
  await page.getByRole('button', { name: 'Start', exact: true }).click()
  const linkField = page.locator('#share-url')
  await expect(linkField).toBeVisible({ timeout: 30_000 })
  const link = await linkField.inputValue()
  expect(link).toMatch(/\/download\/[A-Za-z0-9_-]+#k=[A-Za-z0-9_-]{22,}$/)
  return link
}

test('transfers a file browser to browser', async ({ browser }) => {
  const senderContext = await browser.newContext()
  const receiverContext = await browser.newContext({ acceptDownloads: true })
  const sender = await senderContext.newPage()
  const receiver = await receiverContext.newPage()

  const bytes = payload()
  await sender.goto('/')
  await selectFile(sender, 'chickpeas.bin', bytes)
  const link = await startSending(sender)

  await receiver.goto(link)
  await expect(receiver.getByText('chickpeas.bin')).toBeVisible()
  const downloadPromise = receiver.waitForEvent('download', { timeout: 60_000 })
  await receiver.getByRole('button', { name: 'Start download' }).click()
  const download = await downloadPromise
  const savedPath = await download.path()
  expect(savedPath).toBeTruthy()
  const saved = await readFile(savedPath as string)
  expect(saved.equals(bytes)).toBe(true)

  await expect(receiver.getByText('Download complete')).toBeVisible({ timeout: 60_000 })
  await expect(sender.getByText('Transfer complete', { exact: true })).toBeVisible({
    timeout: 30_000,
  })

  // A completed link is dead.
  const after = await receiverContext.newPage()
  await after.goto(link)
  await expect(after.getByText('Link not found')).toBeVisible()

  await senderContext.close()
  await receiverContext.close()
})

test('password protects the transfer and records wrong attempts', async ({ browser }) => {
  const senderContext = await browser.newContext()
  const receiverContext = await browser.newContext({ acceptDownloads: true })
  const sender = await senderContext.newPage()
  const receiver = await receiverContext.newPage()

  await sender.goto('/')
  await selectFile(sender, 'tahini.txt', Buffer.from('falafel with tahini'))
  await sender.getByRole('switch').click()
  await sender.getByPlaceholder('Password', { exact: true }).fill('tahini')
  const link = await startSending(sender)
  await expect(sender.getByText('Password required')).toBeVisible()

  await receiver.goto(link)
  await expect(receiver.getByText('Password required')).toBeVisible()
  await receiver.getByPlaceholder('Enter password').fill('wrong')
  await receiver.getByRole('button', { name: 'Start download' }).click()
  await expect(receiver.getByText('Wrong password. Try again.')).toBeVisible()
  await expect(sender.getByText('Wrong password attempt')).toBeVisible({ timeout: 20_000 })

  const downloadPromise = receiver.waitForEvent('download', { timeout: 60_000 })
  await receiver.getByPlaceholder('Enter password').fill('tahini')
  await receiver.getByRole('button', { name: 'Start download' }).click()
  const download = await downloadPromise
  expect((await readFile((await download.path()) as string)).toString()).toBe(
    'falafel with tahini',
  )
  await expect(receiver.getByText('Download complete')).toBeVisible({ timeout: 60_000 })
  await expect(sender.getByText('Transfer complete', { exact: true })).toBeVisible()

  await senderContext.close()
  await receiverContext.close()
})

test('refuses a link whose key fragment was stripped', async ({ browser }) => {
  const senderContext = await browser.newContext()
  const receiverContext = await browser.newContext()
  const sender = await senderContext.newPage()
  const receiver = await receiverContext.newPage()

  await sender.goto('/')
  await selectFile(sender, 'saffron.txt', Buffer.from('no key, no file'))
  const link = await startSending(sender)

  await receiver.goto(link.split('#')[0])
  await expect(receiver.getByText('saffron.txt')).toBeVisible()
  await receiver.getByRole('button', { name: 'Start download' }).click()
  await expect(receiver.getByText('This link is incomplete.', { exact: false })).toBeVisible()
  await expect(sender.getByText('Transfer complete', { exact: true })).toBeHidden()

  await senderContext.close()
  await receiverContext.close()
})

test('closing the sender page kills the link', async ({ browser }) => {
  const senderContext = await browser.newContext()
  const sender = await senderContext.newPage()
  await sender.goto('/')
  await selectFile(sender, 'ephemeral.txt', Buffer.from('here for a moment'))
  const link = await startSending(sender)
  await sender.close()

  // Wait for grace period after sender disconnects.
  const visitorContext = await browser.newContext()
  const visitor = await visitorContext.newPage()
  await expect
    .poll(
      async () => {
        // The link ends in a #fragment, so a plain repeat goto would not reload the page.
        await visitor.goto('about:blank')
        await visitor.goto(link)
        return visitor.getByText('Link not found').isVisible()
      },
      { timeout: 20_000, intervals: [1000] },
    )
    .toBe(true)

  await visitorContext.close()
  await senderContext.close()
})

test('unknown links render the 404 view', async ({ page }) => {
  await page.goto('/download/definitely-not-a-real-token')
  await expect(page.getByText('Link not found')).toBeVisible()
})

test('language toggle switches to Persian and persists', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'فا' }).click()
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl')
  await expect(page.getByRole('heading', { level: 1 })).toContainText('ارسال فایل')
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl')
  await expect(page.getByRole('heading', { level: 1 })).toContainText('ارسال فایل')
})

test('follows the device dark theme', async ({ browser }) => {
  const context = await browser.newContext({ colorScheme: 'dark' })
  const page = await context.newPage()
  await page.goto('/')
  const background = await page.evaluate(
    () => getComputedStyle(document.documentElement).backgroundColor,
  )
  expect(background).toBe('rgb(22, 16, 11)')
  await context.close()
})
