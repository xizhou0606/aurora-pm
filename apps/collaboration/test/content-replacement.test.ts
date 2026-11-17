import { afterEach, describe, expect, it } from 'bun:test'
import { HttpAuthorityBridge } from '../src/authority-http.js'
import { ContentReplacement } from '../src/content-replacement.js'
import { documentFromProjection } from '../src/projection.js'
import { AuthorityDouble } from './authority-double.js'

const room = { workspaceId: '7', resourceId: 'doc_1' }
const input = { room, ticket: '', connectionId: '11111111-1111-4111-8111-111111111111', operationId: `tool-${'a'.repeat(64)}`, expectedRevision: 1, content: { type:'doc',content:[{type:'paragraph',content:[{type:'text',text:'正文替换验证'}]}] } }
const authorities: AuthorityDouble[] = []
afterEach(async () => { for (const a of authorities.splice(0)) await a.close() })
async function setup() {
 const a = new AuthorityDouble(); await a.listen(); authorities.push(a)
 const grant = a.addGrant(room,'editor')
 const seed = documentFromProjection({type:'doc',content:[{type:'paragraph'}]}); a.seed(room,seed); seed.destroy()
 const bridge = new HttpAuthorityBridge({baseUrl:a.url,serviceToken:a.serviceToken})
 return { a,bridge,request:{...input,ticket:grant.ticket} }
}
describe('exclusive content replacement', () => {
 it('writes a valid Yjs update and refuses stale revisions',async () => {
  const {bridge,request}=await setup()
  const replace=new ContentReplacement(bridge,()=>false)
  const result=await replace.replace(request)
  expect(result.revision).toBe(2)
  expect(result.content_hash).toMatch(/^[a-f0-9]{64}$/)
  await expect(replace.replace({...request,operationId:`tool-${'b'.repeat(64)}`})).rejects.toMatchObject({code:'DOCUMENT_VERSION_CONFLICT'})
 })
 it('refuses connected/loading rooms before contacting authority',async () => {
  const {bridge,request}=await setup()
  const replace=new ContentReplacement(bridge,()=>true)
  await expect(replace.replace(request)).rejects.toMatchObject({code:'DOCUMENT_EDITING_IN_PROGRESS'})
  expect(replace.active.size).toBe(0)
 })
 it('reserves the room across asynchronous authorization and releases on failure',async () => {
  const {bridge,request}=await setup()
  let release!:()=>void
  const wait=new Promise<void>(resolve=>{release=resolve})
  const original=bridge.authorize.bind(bridge)
  bridge.authorize=async (...args)=>{await wait;return original(...args)}
  const replace=new ContentReplacement(bridge,()=>false)
  const running=replace.replace(request)
  expect(replace.active.size).toBe(1)
  await expect(replace.replace(request)).rejects.toMatchObject({code:'DOCUMENT_REPLACE_IN_PROGRESS'})
  release(); await running; expect(replace.active.size).toBe(0)
 })
})
