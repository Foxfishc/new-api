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
import {
  Activity,
  ArrowDown,
  RefreshCw,
  ShieldCheck,
  WalletCards,
} from 'lucide-react'
import { type ReactNode, useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'

import { getGroupRuleStatus, refreshGroupRuleStatus } from '../api'
import type { GroupRuleStatus } from '../types'

interface GroupRuleStatusCardProps {
  onProfileUpdate: () => void
}

function Metric({
  icon,
  label,
  value,
  description,
}: {
  icon: ReactNode
  label: string
  value: number
  description: string
}) {
  return (
    <div className='bg-muted/30 rounded-xl border p-4'>
      <div className='text-muted-foreground flex items-center gap-2 text-sm'>
        {icon}
        <span>{label}</span>
      </div>
      <div className='text-foreground mt-2 font-mono text-2xl font-semibold tabular-nums'>
        {formatQuotaWithCurrency(value, { digitsLarge: 2, digitsSmall: 4 })}
      </div>
      <div className='text-muted-foreground mt-1 text-xs'>{description}</div>
    </div>
  )
}

export function GroupRuleStatusCard({
  onProfileUpdate,
}: GroupRuleStatusCardProps) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<GroupRuleStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const loadStatus = useCallback(
    async (manual: boolean) => {
      if (manual) setRefreshing(true)
      try {
        const response = manual
          ? await refreshGroupRuleStatus()
          : await getGroupRuleStatus()
        if (response.success && response.data) {
          setStatus(response.data)
          if (response.data.changed) onProfileUpdate()
        } else {
          toast.error(response.message || t('Failed to load group rule status'))
        }
      } catch {
        toast.error(t('Failed to load group rule status'))
      } finally {
        setLoading(false)
        setRefreshing(false)
      }
    },
    [onProfileUpdate, t]
  )

  useEffect(() => {
    void loadStatus(false)
  }, [loadStatus])

  if (loading) {
    return (
      <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
          <Skeleton className='h-6 w-40' />
          <Skeleton className='mt-2 h-4 w-72' />
        </CardHeader>
        <CardContent className='grid gap-3 p-3 sm:grid-cols-2 sm:p-5'>
          <Skeleton className='h-28 rounded-xl' />
          <Skeleton className='h-28 rounded-xl' />
          <Skeleton className='h-24 rounded-xl sm:col-span-2' />
        </CardContent>
      </Card>
    )
  }

  if (!status) return null

  const statusLabel = status.qualified
    ? t('Qualified for {{group}}', { group: status.qualified_group })
    : t('Working toward {{group}}', { group: status.qualified_group })
  const ruleDescription = t(
    'Average consumption or recharge over the last {{days}} days must reach the threshold.',
    { days: status.window_days }
  )

  return (
    <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
      <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
        <div className='flex items-start justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-3'>
            <IconBadge size='title' tone='chart-4'>
              <Activity className='h-4 w-4' />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='text-lg tracking-tight sm:text-xl'>
                {t('Group Rule Status')}
              </CardTitle>
              <p className='text-muted-foreground text-xs sm:text-sm'>
                {t('See your recent averages and automatic group threshold')}
              </p>
            </div>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void loadStatus(true)}
            disabled={refreshing}
            aria-label={t('Refresh group rule status')}
          >
            <RefreshCw className={refreshing ? 'animate-spin' : undefined} />
            <span className='hidden sm:inline'>{t('Refresh')}</span>
          </Button>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 p-3 sm:p-5'>
        <div className='grid gap-3 sm:grid-cols-2'>
          <Metric
            icon={<Activity className='text-muted-foreground h-4 w-4' />}
            label={t('7-day average consumption')}
            value={status.consumption_average_quota}
            description={t('Average charged amount per day')}
          />
          <Metric
            icon={<WalletCards className='text-muted-foreground h-4 w-4' />}
            label={t('7-day average recharge')}
            value={status.recharge_average_quota}
            description={t('Includes redeemed codes')}
          />
        </div>

        <div className='bg-muted/20 rounded-xl border p-4'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='flex items-center gap-2'>
              <Badge variant={status.qualified ? 'default' : 'secondary'}>
                {status.qualified ? <ShieldCheck /> : <ArrowDown />}
                {statusLabel}
              </Badge>
              <span className='text-muted-foreground text-xs'>
                {t('Current group')}: {status.current_group}
              </span>
            </div>
            <span className='text-muted-foreground text-xs'>
              {t('Threshold')}:{' '}
              {formatQuotaWithCurrency(status.threshold_quota)}
            </span>
          </div>
          <p className='text-foreground mt-3 text-sm font-medium'>
            {ruleDescription}
          </p>
          <p className='text-muted-foreground mt-1 text-xs'>
            {status.qualified
              ? t(
                  'The system will keep you in {{group}} while either average meets the threshold.',
                  {
                    group: status.qualified_group,
                  }
                )
              : t(
                  'When the threshold is reached, the system will switch you from {{fallback}} to {{qualified}}.',
                  {
                    fallback: status.fallback_group,
                    qualified: status.qualified_group,
                  }
                )}
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
