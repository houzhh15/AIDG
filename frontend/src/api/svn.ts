import { authedApi } from './auth';

export async function svnSync(){
  const r = await authedApi.post('/svn/sync', {});
  return r.data as { output: string };
}
