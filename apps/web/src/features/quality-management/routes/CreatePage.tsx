import { QUALITY_COPY } from '@/shared/copy'
import { Button, ResponsivePanel } from '@/shared/ui'
import { Create } from '../components'
import type { QualityEnvironment } from '../components/environment'
import { useQualityCreateWorkflow, type QualityScopeState } from '../hooks'
import type { QualityRouteState } from '../model/types'

const CREATE_FORM_ID = 'quality-create-form'

export function CreatePage({
  environment,
  route,
  scope,
}: {
  environment: QualityEnvironment
  route: QualityRouteState
  scope: QualityScopeState
}) {
  const workflow = useQualityCreateWorkflow(
    environment,
    route,
  )
  const pending = workflow.create.state.status === 'pending'
  const formReady = workflow.access === 'allowed' && scope.status === 'ready'
  return (
    <ResponsivePanel
      open
      onOpenChange={(nextOpen) => { if (!nextOpen) void workflow.cancel() }}
      pending={pending}
      title={QUALITY_COPY.form.createTitle}
      closeLabel={QUALITY_COPY.form.cancel}
      className="max-w-[90rem]"
      contentClassName="bg-muted/20"
      footer={(
        <Button
          type="submit"
          form={CREATE_FORM_ID}
          disabled={!formReady || pending}
        >
          {pending ? QUALITY_COPY.mutation.pending : QUALITY_COPY.form.submit}
        </Button>
      )}
    >
      <Create
        formId={CREATE_FORM_ID}
        route={route}
        access={workflow.access}
        scope={scope}
        users={workflow.users}
        mutation={workflow.create.state}
        uploadState={workflow.upload.state}
        canUpload={workflow.canUpload}
        sceneOptions={workflow.sceneOptions}
        sceneOptionMutations={workflow.sceneOptionMutations}
        onSubmit={workflow.submit}
        onUpload={(requirementId, file) => workflow.upload.run({ requirementId, file })}
        onCancel={() => { void workflow.cancel() }}
      />
    </ResponsivePanel>
  )
}
