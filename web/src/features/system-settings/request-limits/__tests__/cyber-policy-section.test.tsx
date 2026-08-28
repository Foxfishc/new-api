/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { CyberPolicySection } from '../cyber-policy-section'

const updateOptionMock = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
  isPending: false,
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => updateOptionMock,
}))

describe('CyberPolicySection', () => {
  beforeEach(() => {
    updateOptionMock.mutateAsync.mockReset()
  })

  it('exposes the automatic-ban switch and configurable event threshold', async () => {
    const user = userEvent.setup()
    render(
      <CyberPolicySection
        defaultValues={{
          CyberAutoBanEnabled: false,
          CyberAutoBanThreshold: 3,
        }}
      />
    )

    const toggle = screen.getByRole('switch', {
      name: 'Enable automatic user bans',
    })
    const threshold = screen.getByRole('spinbutton', {
      name: 'Cyber events before automatic ban',
    })

    expect(toggle).toHaveAttribute('aria-checked', 'false')
    expect(threshold).toHaveValue(3)

    await user.click(toggle)
    await user.clear(threshold)
    await user.type(threshold, '5')

    expect(toggle).toHaveAttribute('aria-checked', 'true')
    expect(threshold).toHaveValue(5)
  })
})
