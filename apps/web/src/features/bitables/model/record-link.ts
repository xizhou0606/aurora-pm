/** A record reference contains only public resource coordinates, never credentials. */
export function bitableRecordPath(scope: { resourceId: string; tableId: string }, viewId: string, recordId: string): string {
  for (const id of [scope.resourceId, scope.tableId, viewId, recordId]) {
    if (!/^[A-Za-z0-9_-]{1,128}$/.test(id)) throw new Error('BITABLE_RECORD_LINK_INVALID')
  }
  const query = new URLSearchParams({ table: scope.tableId, view: viewId, record: recordId })
  return `/workspace/bitables/${scope.resourceId}?${query}`
}
