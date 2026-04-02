#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import config from '../esbuild.config.js';

// 读取 manifest
const manifestPath = path.join(config.paths.build, 'manifest.json');
if (!fs.existsSync(manifestPath)) {
  console.error('✗ manifest.json not found. Run build first.');
  process.exit(1);
}

const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf-8'));

// 获取所有 HTML 文件（从 build/html）
function getHTMLFiles() {
  const htmlDir = config.paths.buildHtml;
  const files = [];

  function walkDir(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walkDir(fullPath);
      } else if (entry.name.endsWith('.html')) {
        files.push(fullPath);
      }
    }
  }

  walkDir(htmlDir);
  return files;
}

console.log('→ Updating HTML templates in build/html...\n');

const htmlFiles = getHTMLFiles();
let updatedCount = 0;

for (const filePath of htmlFiles) {
  let content = fs.readFileSync(filePath, 'utf-8');
  let hasChanges = false;

  // 替换所有 JS 文件引用
  for (const [key, info] of Object.entries(manifest)) {
    if (!info.hash.endsWith('.js')) continue;

    const oldPattern = `assets/${key.replace(/\/\//g, '/')}.min.js`;
    const newPattern = `assets/${info.hash}`;

    // 匹配模式: assets/path/file.js?{{ .cur_ver }}
    const regex = new RegExp(oldPattern.replace(/\//g, '\\/') + '\\?{{\\.cur_ver}}', 'g');

    if (regex.test(content)) {
      content = content.replace(regex, newPattern + '?{{ .cur_ver }}');
      hasChanges = true;
      console.log(`  ✓ Updated ${key} in ${path.relative(config.paths.buildHtml, filePath)}`);
    }
  }

  // 替换所有 CSS 文件引用
  for (const [key, info] of Object.entries(manifest)) {
    if (!info.hash.endsWith('.css')) continue;

    const oldPattern = `assets/${key.replace(/\/\//g, '/')}.min.css`;
    const newPattern = `assets/${info.hash}`;

    const regex = new RegExp(oldPattern.replace(/\//g, '\\/') + '\\?{{\\.cur_ver}}', 'g');

    if (regex.test(content)) {
      content = content.replace(regex, newPattern + '?{{ .cur_ver }}');
      hasChanges = true;
      console.log(`  ✓ Updated ${key} in ${path.relative(config.paths.buildHtml, filePath)}`);
    }
  }

  if (hasChanges) {
    fs.writeFileSync(filePath, content, 'utf-8');
    updatedCount++;
  }
}

console.log(`\n✓ Updated ${updatedCount} HTML file(s) in build/html`);
