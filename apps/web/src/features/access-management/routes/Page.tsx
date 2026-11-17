import { useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { OperationalPage, ViewTabs } from '@/shared/ui'
import { PeopleSection, SubjectsSection } from '../components'

type PageTab = 'people' | 'subjects'

const pageTabs = [
  { id: 'people', label: COPY.page.peopleTab },
  { id: 'subjects', label: COPY.page.subjectsTab },
] as const

export function AccessManagementPage() {
  const [tab, setTab] = useState<PageTab>('people')
  return (
    <OperationalPage
      title={COPY.page.title}
      context={(
        <ViewTabs
          value={tab}
          tabs={pageTabs}
          onValueChange={(value) => setTab(value as PageTab)}
          label={COPY.page.tabsLabel}
        />
      )}
    >
      <div className="mx-auto grid w-full max-w-[96rem] gap-4 p-4 sm:p-5">
        {tab === 'people' ? <PeopleSection /> : <SubjectsSection />}
      </div>
    </OperationalPage>
  )
}
