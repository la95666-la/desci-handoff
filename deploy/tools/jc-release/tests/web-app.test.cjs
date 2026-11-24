const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(__dirname + '/../cmd/jc-release-server/web/app.js', 'utf8');
function fixture(fetch) {
  const elements = new Map();
  const element = id => {
    if (!elements.has(id)) elements.set(id, { value: '', textContent: '', innerHTML: '', classList: { add() {}, remove() {}, toggle() {} } });
    return elements.get(id);
  };
  const context = vm.createContext({ fetch, AbortController, setTimeout, clearTimeout, console, CSS: { escape: x => x },
    window: { location: {} }, document: { readyState: 'loading', visibilityState: 'visible', addEventListener() {},
      getElementById: element, querySelectorAll: () => [], querySelector: () => null } });
  vm.runInContext(source + '\n;globalThis.inspectState=state;', context);
  return { context, element, run: text => vm.runInContext(text, context) };
}
const response = data => ({ ok: true, status: 200, json: async () => ({ status: true, data }) });
test('release list renders without optional overview, failure retains existing data', async () => {
  let fail = false; const urls = [];
  const f = fixture(async path => { urls.push(path); if (fail) throw Error('offline'); return response([{ id:'r', component:'frontend', version:'v1', size:1, sha256:'abc', uploaded_at:new Date().toISOString() }]); });
  await f.run('loadReleases()');
  assert.equal(urls.length, 1); assert.ok(f.element('releases-list').innerHTML.includes('v1'));
  fail = true; await f.run('loadReleases()');
  assert.ok(f.element('releases-list').innerHTML.includes('v1')); assert.match(f.element('releases-status').textContent,/offline/);
});
test('logs use summaries and fetch exact detail only on expansion, escaping response text', async () => {
  const urls=[]; const cmd={id:'cmd',agent_id:'agent',kind:'logs',component:'frontend',status:'success',created_at:new Date().toISOString(),log:'SECRET<log>'};
  const f=fixture(async path=>{urls.push(path); return response(path.includes('summary=true')?[{...cmd,log:''}]:cmd);});
  await f.run('loadLogs()'); assert.equal(urls.length,1);assert.ok(urls[0].endsWith('summary=true'));
  assert.ok(!f.element('logs-list').innerHTML.includes('SECRET'));
  await f.run("toggleLog('cmd')");assert.equal(urls[1],'/api/v1/commands/cmd');
  await f.run('loadLogs()');assert.ok(f.element('logs-list').innerHTML.includes('SECRET&lt;log&gt;'));
});
test('same GET coalesces while in flight', async () => {
  let resolve,calls=0;
  const f=fixture(()=>{calls++;return new Promise(r=>resolve=r)});
  const a=f.run("readOnce('/one')"),b=f.run("readOnce('/one')");assert.equal(calls,1);
  resolve(response([]));await Promise.all([a,b]);
  assert.equal(f.context.inspectState.requests.size,0);
});
test('poll loop is completion-based, no overlapping setInterval or hidden full logs',()=>{
  assert.ok(!source.includes('setInterval('));assert.match(source,/finally \{ startPolling\(\); \}/);
  assert.ok(!source.includes("esc(c.log ||"));
});

test('upgrade renders Pending immediately, reload restores progress, completion renders success without resend', async () => {
  let status='pending', posts=0;
  const command=()=>({id:'upgrade-one',agent_id:'agent',kind:'upgrade',component:'frontend',release_id:'release',status,created_at:new Date().toISOString()});
  const f=fixture(async (path,options)=>{
    if(options?.method==='POST'){posts++;return response(command())}
    return response([command()]);
  });
  f.context.confirm=()=>true;
  f.run("state.agents=[{id:'agent',hostname:'host',deployed:{}}];state.latestPerComp.frontend={id:'release'}");
  await f.run("upgradeAgent('agent','frontend','release')");
  assert.equal(posts,1);
  assert.match(f.element('agents-list').innerHTML,/pill pending/);
  await f.run("upgradeAgent('agent','frontend','release')");
  assert.equal(posts,1);
  status='in_progress';
  f.run('state.upgradeCommands=[]'); // simulate restored page with no local commands
  await f.run('loadUpgradeStatus()');
  assert.match(f.element('agents-list').innerHTML,/pill in_progress/);
  status='success';await f.run('loadUpgradeStatus()');
  assert.match(f.element('agents-list').innerHTML,/pill success/);
  assert.equal(posts,1);
});
