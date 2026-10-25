import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import PasswordField from '@/components/PasswordField.vue'
import { i18n } from '@/i18n'

function mountField() {
  return mount(PasswordField, {
    props: { modelValue: '', placeholder: 'Password' },
    global: { plugins: [i18n] },
  })
}

describe('PasswordField', () => {
  it('starts masked and toggles visibility', async () => {
    const wrapper = mountField()
    const input = wrapper.get('input')
    expect(input.attributes('type')).toBe('password')

    await wrapper.get('button').trigger('click')
    expect(wrapper.get('input').attributes('type')).toBe('text')
    expect(wrapper.get('button').attributes('aria-label')).toBe('Hide')
  })

  it('emits typed values', async () => {
    const wrapper = mountField()
    await wrapper.get('input').setValue('tahini')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['tahini'])
  })

  it('applies the minimum length', () => {
    const wrapper = mount(PasswordField, {
      props: { modelValue: '', placeholder: 'Password', minLength: 6 },
      global: { plugins: [i18n] },
    })
    expect(wrapper.get('input').attributes('minlength')).toBe('6')
  })
})
