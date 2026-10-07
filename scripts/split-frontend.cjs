const fs = require('node:fs');
const path = require('node:path');
const root = path.join(__dirname, '../internal/manager/web');
const file = path.join(root, 'index.html');
let html = fs.readFileSync(file, 'utf8');
const css = html.match(/<style>([\s\S]*?)<\/style>/);
const js = html.match(/<script>([\s\S]*?)<\/script>/);
if (css && js) {
  fs.mkdirSync(path.join(root, 'assets'), {recursive:true});
  fs.writeFileSync(path.join(root, 'assets/style.css'), css[1]);
  fs.writeFileSync(path.join(root, 'assets/app.js'), js[1]);
  html = html.replace(css[0], '<link rel="stylesheet" href="__PANEL_PATH__/assets/style.css">')
    .replace(js[0], '<script src="__PANEL_PATH__/assets/app.js" defer></script>');
  fs.writeFileSync(file, html);
}
