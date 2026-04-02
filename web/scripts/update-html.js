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

// 创建资源映射表（原始文件名 -> 带哈希的文件名）
const resourceMap = {};

for (const [key, info] of Object.entries(manifest)) {
  // key 格式: "vue/vue.min.js", "ant-design-vue/antd.min.js", "js/axios-init.js", "css/custom.min.css"
  // info.hash 格式: "vue/vue.min.js.min.hash.js", "antd.min.js.min.hash.js", "custom.min.css.min.hash.css"
  // HTML 中的引用格式:
  //   - "assets/vue/vue.min.js"
  //   - "assets/ant-design-vue/antd.min.css"
  //   - "assets/js/axios-init.js"

  const hashedPath = `assets/${info.hash}`;
  const originalPath = `assets/${key}`;

  // 主映射
  resourceMap[originalPath] = hashedPath;

  // 对于 JS 文件，创建不带 .min.js 的映射
  if (key.endsWith('.js')) {
    const baseKey = key.replace(/\.min\.js$/, '.js').replace(/\.js$/, '');
    resourceMap[`assets/${baseKey}`] = hashedPath;
    resourceMap[`assets/${baseKey}.min.js`] = hashedPath;
    resourceMap[`assets/${baseKey}.js`] = hashedPath;
  }

  // 对于 CSS 文件，创建不带 .min.css 的映射
  if (key.endsWith('.css')) {
    const baseKey = key.replace(/\.min\.css$/, '.css').replace(/\.css$/, '');
    resourceMap[`assets/${baseKey}`] = hashedPath;
    resourceMap[`assets/${baseKey}.min.css`] = hashedPath;
    resourceMap[`assets/${baseKey}.css`] = hashedPath;
  }
}

// 调试：检查一些关键映射
console.log('Sample resource mappings:');
const samples = ['assets/vue/vue.min.js', 'assets/vue/vue.min', 'assets/js/axios-init.js', 'assets/js/axios-init'];
for (const sample of samples) {
  console.log(`  ${sample} -> ${resourceMap[sample] || 'NOT FOUND'}`);
}
console.log();

for (const filePath of htmlFiles) {
  let content = fs.readFileSync(filePath, 'utf-8');
  let hasChanges = false;

  // 替换所有资源引用
  for (const [oldPattern, newPattern] of Object.entries(resourceMap)) {
    // 匹配模式:
    // - assets/path/file.js?{{ .cur_ver }}
    // - assets/path/file.js
    // - assets/path/file.css?{{ .cur_ver }}
    // - assets/path/file.css

    const regex1 = new RegExp(oldPattern.replace(/[\/\.]/g, '\\$&') + '\\?\\{\\{ \\.cur_ver \\}\\}', 'g');
    // 匹配路径后面跟着引号、空格或 > 的情况
    const regex2 = new RegExp(oldPattern.replace(/[\/\.]/g, '\\$&') + '([\\s>"\'])', 'g');

    let replaced = false;

    // 首先尝试匹配带 ?{{ .cur_ver }} 的版本
    if (regex1.test(content)) {
      content = content.replace(regex1, `${newPattern}?{{ .cur_ver }}`);
      replaced = true;
    } else {
      // 如果没有找到，尝试匹配不带版本参数的版本
      content = content.replace(regex2, (match, suffix) => {
        // 检查是否已经包含哈希（避免重复替换）
        if (!match.match(/\.[a-f0-9]{60,}\.(js|css)/)) {
          replaced = true;
          return `${newPattern}${suffix}`;
        }
        return match;
      });
    }

    if (replaced) {
      hasChanges = true;
      const relativePath = path.relative(config.paths.buildHtml, filePath);
      console.log(`  ✓ Updated ${oldPattern} in ${relativePath}`);
    }
  }

  if (hasChanges) {
    fs.writeFileSync(filePath, content, 'utf-8');
    updatedCount++;
  }
}

console.log(`\n✓ Updated ${updatedCount} HTML file(s) in build/html`);
