const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');

const html = fs.readFileSync(path.join(__dirname, '../internal/manager/web/index.html'), 'utf8');
const script = fs.readFileSync(path.join(__dirname, '../internal/manager/web/assets/app.js'), 'utf8');
new vm.Script(script);
new vm.Script(fs.readFileSync(path.join(__dirname, '../internal/manager/web/assets/app-enhancements.js'), 'utf8'));
const names = [...script.matchAll(/(?:async\s+)?function\s+(\w+)\s*\(/g)].map(match => match[1]);
assert.equal(new Set(names).size, names.length, 'Duplicate named functions can silently overwrite fixes');

const dom = {
  getElementById: () => ({ innerHTML: '', classList: { add() {}, remove() {} } }),
  documentElement: { classList: { toggle() {} } },
};
const context = vm.createContext({
  document: dom, location: { pathname: '/panel/test', origin: 'http://localhost', hash: '' },
  localStorage: { getItem: () => 'light' }, window: { addEventListener() {} },
  console, setTimeout, clearTimeout,
});
// Skip startup HTTP requests while exercising the actual render functions.
vm.runInContext(script.replace(/\binit\(\);\s*$/, ''), context);
const row = vm.runInContext(`portRow({id:'p1',port:1234,enabled:true,instanceRunning:true},'mierus://u:p@host','n1')`, context);
assert.ok(row.includes('运行中'));
assert.ok(row.includes('data-port-actions="p1"'));
assert.ok(row.includes('value="traffic"'));
assert.ok(row.includes('value="retry"'));
assert.ok(row.includes('value="regenerate"'));
assert.ok(row.includes('value="delete"'));
assert.ok(row.includes('port-detail-p1'));
assert.ok(row.includes('data-copy="mierus://u:p@host"'));
assert.ok(row.includes('复制链接'));
for (const type of ['naive', 'anytls', 'shadowtls']) {
  const unsupported = vm.runInContext(`portRow({id:'p2',port:443,proxyType:'${type}',enabled:true},'test://node','n1')`, context);
  assert.ok(unsupported.includes('暂不支持流量统计'));
  assert.ok(!unsupported.includes('value="traffic"'));
  assert.ok(!unsupported.includes('已用 0'));
}
const detail = vm.runInContext(`detailView({id:'n1',name:'Tokyo',ports:[{id:'p1',port:1234,protocol:'TCP',username:'private-user',enabled:true}]},{},[])`, context);
assert.ok(detail.includes('② 代理入口'));
assert.ok(!detail.includes('<th>账号</th>'));
assert.ok(!detail.includes('private-user'));
assert.ok(detail.includes('colspan="6"'));
const health = vm.runInContext(`healthPanel({registered:true,online:false,lastSeen:'2026-01-01T00:00:00Z',mitaRunning:false,ports:[{enabled:true,instanceRunning:false}]})`, context);
assert.ok(health.includes('Agent 心跳'));
assert.ok(health.includes('需要关注'));
const audit = vm.runInContext(`auditTable([{time:'2026-01-01T00:00:00Z',actor:'admin',action:'新增节点',target:'edge-a'}])`, context);
assert.ok(audit.includes('新增节点'));
vm.runInContext(`state.nodes=[{name:'Tokyo',address:'192.0.2.1',online:true},{name:'Berlin',registered:true,online:false}];nodeFilter.query='192.0.2';nodeFilter.status='online'`, context);
assert.equal(vm.runInContext('filteredNodes().length', context), 1);
vm.runInContext(`nodeFilter.status='offline'`, context);
assert.equal(vm.runInContext('filteredNodes().length', context), 0);
const cloud = vm.runInContext(`cloudBackupPanel({url:'https://cloud.example/备份/',username:'<script>bad</script>',hasPassword:true,allowHTTP:false})`, context);
assert.ok(cloud.includes('WebDAV 云备份'));
assert.ok(cloud.includes('cloudBackupForm'));
assert.ok(cloud.includes('data-cloud-action="upload"'));
assert.ok(cloud.includes('data-cloud-action="list"'));
assert.ok(cloud.includes('&lt;script&gt;'));
assert.ok(!cloud.includes('<script>bad</script>'));
assert.ok(!cloud.includes('新增同步账号'));
const files = vm.runInContext(`cloudBackupFilesTable([{name:'<bad>',size:100,modified:''}])`, context);
assert.ok(files.includes('data-cloud-restore="&lt;bad&gt;"'));
assert.ok(!files.includes('<bad>'));
const fields = {
  '[name="proxyType"]': { value: 'mieru' },
  '[name="port"]': { value: '' },
  '[name="protocol"]': { value: 'UDP' },
  '[name="domain"]': {},
  '[name="autoACME"]': { checked: true },
  '#naiveFields': { style: {} },
  '#shadowtlsFields': { style: {} },
  '#manualCertFields': { style: {} },
  '#portRangeLabel': {},
};
context.deploymentForm = { querySelector: selector => {
  assert.ok(fields[selector], `Unexpected form selector: ${selector}`);
  return fields[selector];
} };
vm.runInContext('bindProxyPortForm(deploymentForm)', context);
assert.equal(fields['[name="port"]'].min, '1025');
assert.equal(fields['[name="protocol"]'].disabled, false);
fields['[name="proxyType"]'].value = 'naive';
fields['[name="proxyType"]'].onchange();
assert.equal(fields['[name="port"]'].min, '1');
assert.equal(fields['[name="port"]'].value, '443');
assert.equal(fields['[name="protocol"]'].disabled, true);
assert.equal(fields['[name="protocol"]'].value, 'TCP');
assert.equal(fields['#naiveFields'].style.display, 'block');
assert.equal(fields['[name="domain"]'].required, true);
fields['[name="autoACME"]'].checked = false;
fields['[name="autoACME"]'].onchange();
assert.equal(fields['#manualCertFields'].style.display, 'block');
fields['[name="proxyType"]'].value = 'shadowtls';
fields['[name="proxyType"]'].onchange();
assert.equal(fields['#shadowtlsFields'].style.display, 'block');
assert.equal(fields['[name="domain"]'].required, false);
fields['[name="proxyType"]'].value = 'mieru';
fields['[name="proxyType"]'].onchange();
assert.equal(fields['[name="port"]'].value, '');
assert.equal(fields['[name="protocol"]'].disabled, false);
assert.ok(!script.includes('<script>'), 'Modal controls must be bound after mounting; innerHTML does not execute scripts');
console.log('Frontend syntax, node rendering, protocol forms and WebDAV cloud backup rendering passed');
