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
assert.ok(row.includes('data-retry-port="p1"'));
assert.ok(row.includes('port-detail-p1'));
assert.ok(row.includes('复制 Mieru'));
vm.runInContext(`state.nodes=[{name:'Tokyo',address:'192.0.2.1',online:true},{name:'Berlin',registered:true,online:false}];nodeFilter.query='192.0.2';nodeFilter.status='online'`, context);
assert.equal(vm.runInContext('filteredNodes().length', context), 1);
vm.runInContext(`nodeFilter.status='offline'`, context);
assert.equal(vm.runInContext('filteredNodes().length', context), 0);
console.log('Frontend syntax, unique definitions, instance status and expand/retry rendering passed');
