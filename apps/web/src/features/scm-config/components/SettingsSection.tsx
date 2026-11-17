import { useEffect, useMemo, useState } from 'react'

import { PhSlidersHorizontal } from '@/shared/ui/icons'
import {
  AlertBanner,
  Button,
  DetailSection,
  Input,
  Label,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmSettingsDTO } from '../api/contracts'
import {
  useSaveSettingsMutation,
  useScmConfigAccess,
  useScmConfigError,
  useSettingsQuery,
} from '../hooks'

type SettingsFormState = {
  page_cap: string
}

type ScheduleState = {
  daily_sync_hour: number
  daily_sync_minute: number
  weekly_sync_weekday: number
  weekly_sync_hour: number
  weekly_sync_minute: number
  monthly_sync_day: number
  monthly_sync_hour: number
  monthly_sync_minute: number
}

const HOURLY_ITEMS = Array.from({ length: 24 }, (_, hour) => ({
  value: String(hour),
  label: COPY.settings.schedule.timeLabel(hour, 0).slice(0, 2),
}))

// 每月触发日上限 28：二月没有 29/30/31 号，避免触发日永远跳过。
const MONTHLY_ITEMS = Array.from({ length: 28 }, (_, index) => ({
  value: String(index + 1),
  label: COPY.settings.schedule.dayLabel(index + 1),
}))

const MINUTE_ITEMS = Array.from({ length: 60 }, (_, minute) => ({
  value: String(minute),
  label: String(minute).padStart(2, '0'),
}))

function weekdayItems() {
  const names = COPY.settings.schedule.weekdayNames
  // 触发日按周一→周日排序（周日=0 放在最后），与工作周习惯一致。
  return [1, 2, 3, 4, 5, 6, 0].map((value) => ({ value: String(value), label: names[value] }))
}

const SUMMARY_LABEL_CLASS = 'text-xs text-muted-foreground'
const SUMMARY_VALUE_CLASS = 'mt-0.5 text-sm font-medium'

export function SettingsSection() {
  const access = useScmConfigAccess()
  const canView = access.canView('settings')
  const canManage = access.canManage('settings')
  const settingsQuery = useSettingsQuery(canView)
  const { error, reportError, clearError } = useScmConfigError()
  const [saved, setSaved] = useState(false)
  const [form, setForm] = useState<SettingsFormState | null>(null)
  const [schedule, setSchedule] = useState<ScheduleState | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  const settings = settingsQuery.data
  useEffect(() => {
    if (settings && form === null) {
      setForm({ page_cap: String(settings.page_cap) })
      setSchedule({
        daily_sync_hour: settings.daily_sync_hour,
        daily_sync_minute: settings.daily_sync_minute,
        weekly_sync_weekday: settings.weekly_sync_weekday,
        weekly_sync_hour: settings.weekly_sync_hour,
        weekly_sync_minute: settings.weekly_sync_minute,
        monthly_sync_day: settings.monthly_sync_day,
        monthly_sync_hour: settings.monthly_sync_hour,
        monthly_sync_minute: settings.monthly_sync_minute,
      })
    }
  }, [settings, form])

  const saveMutation = useSaveSettingsMutation(reportError)
  const weekdayItemsMemo = useMemo(weekdayItems, [])

  if (!canView || settingsQuery.isPending || form === null || schedule === null) {
    return null
  }

  const submit = () => {
    clearError()
    setSaved(false)
    saveMutation.mutate(
      {
        page_cap: Number(form.page_cap),
        daily_sync_hour: schedule.daily_sync_hour,
        daily_sync_minute: schedule.daily_sync_minute,
        weekly_sync_weekday: schedule.weekly_sync_weekday,
        weekly_sync_hour: schedule.weekly_sync_hour,
        weekly_sync_minute: schedule.weekly_sync_minute,
        monthly_sync_day: schedule.monthly_sync_day,
        monthly_sync_hour: schedule.monthly_sync_hour,
        monthly_sync_minute: schedule.monthly_sync_minute,
      },
      {
        onSuccess: (config: ScmSettingsDTO) => {
          setSchedule({
            daily_sync_hour: config.daily_sync_hour,
            daily_sync_minute: config.daily_sync_minute,
            weekly_sync_weekday: config.weekly_sync_weekday,
            weekly_sync_hour: config.weekly_sync_hour,
            weekly_sync_minute: config.weekly_sync_minute,
            monthly_sync_day: config.monthly_sync_day,
            monthly_sync_hour: config.monthly_sync_hour,
            monthly_sync_minute: config.monthly_sync_minute,
          })
          setForm((prev) => (prev ? { ...prev, page_cap: String(config.page_cap) } : prev))
          setSaved(true)
          setDialogOpen(false)
        },
      },
    )
  }

  const scheduleSummary = COPY.settings.schedule.summary({
    dailyTime: COPY.settings.schedule.timeLabel(schedule.daily_sync_hour, schedule.daily_sync_minute),
    weekday: COPY.settings.schedule.weekdayNames[schedule.weekly_sync_weekday] ?? '一',
    weeklyTime: COPY.settings.schedule.timeLabel(schedule.weekly_sync_hour, schedule.weekly_sync_minute),
    monthDay: COPY.settings.schedule.dayLabel(schedule.monthly_sync_day),
    monthlyTime: COPY.settings.schedule.timeLabel(schedule.monthly_sync_hour, schedule.monthly_sync_minute),
  })

  return (
    <DetailSection
      className="overflow-hidden"
      title={(
        <span className="flex items-center gap-2">
          <PhSlidersHorizontal size={16} tone="neutral" aria-hidden="true" />
          {COPY.settings.title}
        </span>
      )}
      description={COPY.settings.description}
      actions={canManage ? <Button onClick={() => setDialogOpen(true)}>{COPY.settings.edit}</Button> : undefined}
    >
      <div className="grid gap-3">
        {error ? (
          <AlertBanner tone="error" onDismiss={clearError}>
            {COPY.errors.requestFailed(error)}
          </AlertBanner>
        ) : null}
        {saved ? (
          <AlertBanner tone="success" onDismiss={() => setSaved(false)}>
            {COPY.common.savedMessage}
          </AlertBanner>
        ) : null}
        <div className="grid gap-x-6 gap-y-4 rounded-lg border bg-muted/40 p-4 sm:grid-cols-3">
          <div className="min-w-0">
            <p className={SUMMARY_LABEL_CLASS}>{COPY.settings.schedule.label}</p>
            <p className={SUMMARY_VALUE_CLASS}>{scheduleSummary}</p>
          </div>
          <div className="min-w-0">
            <p className={SUMMARY_LABEL_CLASS}>{COPY.settings.pageCapLabel}</p>
            <p className={SUMMARY_VALUE_CLASS}>{COPY.common.pageCapSummary(form.page_cap)}</p>
          </div>
        </div>
      </div>
      {canManage ? (
        <ResponsivePanel
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          title={COPY.settings.dialogTitle}
          description={COPY.settings.dialogDescription}
          closeLabel={COPY.common.close}
          footer={
            <>
              <Button variant="outline" onClick={() => setDialogOpen(false)}>
                {COPY.common.cancel}
              </Button>
              <Button disabled={saveMutation.isPending} onClick={submit}>
                {COPY.common.save}
              </Button>
            </>
          }
        >
          <div className="grid gap-5">
            <fieldset className="grid gap-3">
              <legend className="mb-1 text-sm font-medium">{COPY.settings.schedule.label}</legend>
              <label className="grid gap-1.5">
                <span className="text-xs text-muted-foreground">{COPY.settings.schedule.daily}</span>
                <div className="flex items-center gap-1.5">
                  <Select
                    items={[...HOURLY_ITEMS]}
                    value={String(schedule.daily_sync_hour)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, daily_sync_hour: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.daily} className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {HOURLY_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <span className="text-muted-foreground">{COPY.settings.schedule.timeSeparator}</span>
                  <Select
                    items={MINUTE_ITEMS}
                    value={String(schedule.daily_sync_minute)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, daily_sync_minute: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.minuteAria(COPY.settings.schedule.daily)} className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MINUTE_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </label>
              <label className="grid gap-1.5">
                <span className="text-xs text-muted-foreground">{COPY.settings.schedule.weekly}</span>
                <div className="flex items-center gap-1.5">
                  <Select
                    items={weekdayItemsMemo}
                    value={String(schedule.weekly_sync_weekday)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, weekly_sync_weekday: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.weekly} className="w-28">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {weekdayItemsMemo.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Select
                    items={[...HOURLY_ITEMS]}
                    value={String(schedule.weekly_sync_hour)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, weekly_sync_hour: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.hourAria(COPY.settings.schedule.weekly)} className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {HOURLY_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <span className="text-muted-foreground">{COPY.settings.schedule.timeSeparator}</span>
                  <Select
                    items={MINUTE_ITEMS}
                    value={String(schedule.weekly_sync_minute)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, weekly_sync_minute: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.minuteAria(COPY.settings.schedule.weekly)} className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MINUTE_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </label>
              <label className="grid gap-1.5">
                <span className="text-xs text-muted-foreground">{COPY.settings.schedule.monthly}</span>
                <div className="flex items-center gap-1.5">
                  <Select
                    items={[...MONTHLY_ITEMS]}
                    value={String(schedule.monthly_sync_day)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, monthly_sync_day: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.monthly} className="w-28">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MONTHLY_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Select
                    items={[...HOURLY_ITEMS]}
                    value={String(schedule.monthly_sync_hour)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, monthly_sync_hour: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.hourAria(COPY.settings.schedule.monthly)} className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {HOURLY_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <span className="text-muted-foreground">{COPY.settings.schedule.timeSeparator}</span>
                  <Select
                    items={MINUTE_ITEMS}
                    value={String(schedule.monthly_sync_minute)}
                    onValueChange={(value) =>
                      setSchedule((prev) => (prev ? { ...prev, monthly_sync_minute: Number(value) } : prev))}
                  >
                    <SelectTrigger aria-label={COPY.settings.schedule.minuteAria(COPY.settings.schedule.monthly)} className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MINUTE_ITEMS.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </label>
            </fieldset>
            <div className="grid gap-4">
              <label className="grid gap-1.5">
                <Label htmlFor="scm-settings-page-cap">{COPY.settings.pageCapLabel}</Label>
                <Input
                  id="scm-settings-page-cap"
                  inputMode="numeric"
                  value={form.page_cap}
                  onChange={(event) => setForm((prev) => (prev ? { ...prev, page_cap: event.target.value } : prev))}
                />
              </label>
            </div>
          </div>
        </ResponsivePanel>
      ) : null}
    </DetailSection>
  )
}
