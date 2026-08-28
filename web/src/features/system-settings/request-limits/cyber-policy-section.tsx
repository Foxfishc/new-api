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
import { zodResolver } from '@hookform/resolvers/zod'
import { ShieldAlert } from 'lucide-react'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const cyberPolicySchema = z.object({
  CyberAutoBanEnabled: z.boolean(),
  CyberAutoBanThreshold: z.number().int().min(0).max(1000000),
})

type CyberPolicyFormValues = z.infer<typeof cyberPolicySchema>

type CyberPolicySectionProps = {
  defaultValues: CyberPolicyFormValues
}

export function CyberPolicySection({
  defaultValues,
}: CyberPolicySectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<CyberPolicyFormValues>({
    resolver: zodResolver(cyberPolicySchema),
    mode: 'onChange',
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: CyberPolicyFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof CyberPolicyFormValues]
    )
    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  return (
    <SettingsSection title={t('Cyber Policy')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save Cyber policy settings'
          />
          <FormField
            control={form.control}
            name='CyberAutoBanEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel className='flex items-center gap-2'>
                    <ShieldAlert className='h-4 w-4' />
                    {t('Enable automatic user bans')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Disable a user after they reach the configured number of upstream cyber-policy events.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='CyberAutoBanThreshold'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Cyber events before automatic ban')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={1000000}
                    step={1}
                    {...field}
                    onChange={(event) =>
                      field.onChange(Number.parseInt(event.target.value) || 0)
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The count is cumulative per user. Set to 0 to disable the threshold; administrators are never automatically banned.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
