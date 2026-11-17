import { GuideActions } from './GuideActions'
import { useProjectAccessGuide } from '../hooks/useProjectAccessGuide'
import type { ProjectGuideTarget } from '../api/project-guide'
import { Button, CopyLinkButton } from '@/shared/ui'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'

export function ProjectAccessGuide({ title, denied = false, sharePath, target }: {
  target?: ProjectGuideTarget
  title: string
  denied?: boolean
  sharePath?: string
}) {
  const copy = RESOURCE_ACCESS_COPY.projectGuide
  return (
    <section role="alert" className="mx-auto grid max-w-xl gap-4 px-6 py-12">
      <h2 className="text-lg font-semibold">{denied ? copy.deniedTitle : title}</h2>
      <p className="text-sm text-muted-foreground">{denied ? copy.denied : copy.unavailable}</p>
      {target ? <ProjectGuideMetadata target={target} /> : null}
      <ol className="list-decimal space-y-3 pl-5 text-sm">
        <li>{copy.workspace}</li>
        <li>{copy.request}</li>
        <li>{copy.afterApproval}</li>
      </ol>
      {sharePath ? <div><CopyLinkButton path={sharePath} /></div> : null}
    </section>
  )
}

function ProjectGuideMetadata({ target }: { target: ProjectGuideTarget }) {
  const flow = useProjectAccessGuide(target)
  const copy = RESOURCE_ACCESS_COPY.projectGuide
  if (flow.status === 'loading') return <p role="status">{copy.loading}</p>
  if (flow.status === 'missing') return <p>{copy.missing}</p>
  if (flow.status === 'error') return <div><p>{copy.failed}</p><Button onClick={flow.retry}>{copy.retry}</Button></div>
  if (!flow.data) return null
  return <><dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
    <dt>{copy.workspaceLabel}</dt><dd>{flow.data.workspaceName}</dd>
    <dt>{copy.projectSetLabel}</dt><dd>{flow.data.projectSetName}</dd>
    <dt>{copy.projectLabel}</dt><dd>{flow.data.projectName}</dd>
    <dt>{copy.ownerLabel}</dt><dd>{flow.data.ownerName || copy.noOwner}</dd>
  </dl><GuideActions memberships={flow.data.memberships} /></>
}
