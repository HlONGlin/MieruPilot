const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');

const html = fs.readFileSync(path.join(__dirname, '../internal/manager/web/index.html'), 'utf8');
const script = fs.readFileSync(path.join(__dirname, '../internal/manager/web/assets/app.js'), 'utf8');
new vm.Script(script);
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
assert.ok(row.includes('复制 Mieru'));
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
console.log('Frontend syntax, unique definitions, instance status and expand/retry rendering passed');
