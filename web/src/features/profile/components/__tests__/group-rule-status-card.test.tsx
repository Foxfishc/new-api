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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { GroupRuleStatusCard } from '../group-rule-status-card'

const apiMocks = vi.hoisted(() => ({
  getGroupRuleStatus: vi.fn(),
  refreshGroupRuleStatus: vi.fn(),
}))

const i18nMock = vi.hoisted(() => ({
  t: (key: string, values?: Record<string, unknown>) => {
    if (!values) return key
    return Object.entries(values).reduce(
      (text, [name, value]) => text.replace(`{{${name}}}`, String(value)),
      key
    )
  },
}))

vi.mock('../../api', () => apiMocks)

vi.mock('react-i18next', () => ({
  useTranslation: () => i18nMock,
}))

vi.mock('@/lib/currency', () => ({
  formatQuotaWithCurrency: (value: number) => `quota:${value}`,
}))

const baseStatus = {
  user_id: 1,
  current_group: 'default',
  qualified: false,
  changed: false,
  consumption_average: 3,
  recharge_average: 5,
  consumption_average_quota: 1_500_000,
  recharge_average_quota: 2_500_000,
  threshold_quota: 5_000_000,
  window_days: 7,
  currency: 'USD',
  currency_symbol: '$',
  qualified_group: 'svip',
  fallback_group: 'default',
  evaluated_at: 1,
}

describe('GroupRuleStatusCard', () => {
  beforeEach(() => {
    apiMocks.getGroupRuleStatus.mockReset()
    apiMocks.refreshGroupRuleStatus.mockReset()
  })

  it('shows both seven-day averages and explains redemption credits', async () => {
    apiMocks.getGroupRuleStatus.mockResolvedValue({
      success: true,
      data: baseStatus,
    })

    render(<GroupRuleStatusCard onProfileUpdate={vi.fn()} />)

    expect(await screen.findByText('7-day average consumption')).toBeVisible()
    expect(screen.getByText('quota:1500000')).toBeVisible()
    expect(screen.getByText('7-day average recharge')).toBeVisible()
    expect(screen.getByText('quota:2500000')).toBeVisible()
    expect(screen.getByText('Includes redeemed codes')).toBeVisible()
    expect(screen.getByText('Working toward svip')).toBeVisible()
  })

  it('refreshes the rule and updates the profile after a group change', async () => {
    const onProfileUpdate = vi.fn()
    apiMocks.getGroupRuleStatus.mockResolvedValue({
      success: true,
      data: baseStatus,
    })
    apiMocks.refreshGroupRuleStatus.mockResolvedValue({
      success: true,
      data: {
        ...baseStatus,
        current_group: 'svip',
        qualified: true,
        changed: true,
      },
    })

    render(<GroupRuleStatusCard onProfileUpdate={onProfileUpdate} />)
    await screen.findByText('Working toward svip')

    await userEvent.click(
      screen.getByRole('button', { name: 'Refresh group rule status' })
    )

    await waitFor(() => {
      expect(apiMocks.refreshGroupRuleStatus).toHaveBeenCalledTimes(1)
      expect(onProfileUpdate).toHaveBeenCalledTimes(1)
      expect(screen.getByText('Qualified for svip')).toBeVisible()
    })
  })
})
