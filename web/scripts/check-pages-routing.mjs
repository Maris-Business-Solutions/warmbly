import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');
const rules = read('../public/_redirects').split('\n').filter((line) => line.trim() && !line.startsWith('#')).map((line) => {
  const [source, target, code] = line.trim().split(/\s+/);
  return { source, target, code };
});
const match = (path) => rules.find(({ source }) => source.endsWith('*') ? path.startsWith(source.slice(0, -1)) : path === source);
const routes = ts.createSourceFile('router.tsx', read('../src/router.tsx'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const roots = [];
const visit = (node) => {
  if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
    if (node.expression.text === 'standalone' && ts.isStringLiteral(node.arguments[0])) roots.push('/' + node.arguments[0].text);
    if (node.expression.text === 'createRoute' && ts.isObjectLiteralExpression(node.arguments[0])) {
      const properties = node.arguments[0].properties;
      const parent = properties.find((p) => p.name?.getText(routes) === 'getParentRoute');
      const path = properties.find((p) => p.name?.getText(routes) === 'path');
      if (parent && ts.isPropertyAssignment(parent) && ts.isArrowFunction(parent.initializer)
          && parent.initializer.body.getText(routes) === 'rootRoute'
          && path && ts.isPropertyAssignment(path) && ts.isStringLiteral(path.initializer)) {
        roots.push(path.initializer.text === '/' ? '/' : '/' + path.initializer.text);
      }
    }
  }
  ts.forEachChild(node, visit);
};
visit(routes);
assert.ok(roots.length > 10, 'root route discovery did not find the route table');
for (const path of [...roots, '/auth/login/confirm', '/auth/reset-password/confirm', '/app/campaigns/a/preferences', '/app/unibox/inbox/thread', '/app/settings/security']) {
  const rule = match(path);
  assert.equal(rule?.target, '/index.html', `Pages does not serve SPA route ${path}`);
  assert.equal(rule?.code, '200');
}
for (const path of ['/assets/missing.js', '/assets/layout-aTnJVElG.js', '/config.js', '/favicon.ico', '/mail-preview.html', '/missing-resource']) {
  assert.equal(match(path), undefined, `${path} must not rewrite to HTML`);
}
assert.match(read('../public/404.html'), /<!doctype html>/i);
assert.doesNotMatch(read('../public/_headers'), /Cache-Control:.*immutable/);
console.log('Pages SPA deep links and missing-asset separation passed');
