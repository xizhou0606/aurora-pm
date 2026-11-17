import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { OperationalPage } from '@/shared/ui'
import { WorkspacesSection } from '../components'

export function WorkspaceManagementPage() {
  return (
    <OperationalPage
      title={COPY.workspaces.sectionLabel}
    >
      <div className="mx-auto grid w-full max-w-[96rem] gap-4 p-4 sm:p-5">
        <WorkspacesSection />
      </div>
    </OperationalPage>
  )
}
