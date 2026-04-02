# esbuild 前端资源压缩和哈希方案

## Context

当前项目的前端资源（JS/CSS）在 `web/src/assets` 目录中，总计 41 个文件约 8.3MB。部分文件已压缩，部分未压缩（如 `inbound.js` 82KB）。资源在编译时通过 `//go:embed` 嵌入到 Go 二进制文件中，使用查询参数 `?{{ .cur_ver }}` 实现缓存管理。

**问题**：
- 文件体积较大，影响加载速度
- 缺少内容哈希，缓存策略不够高效
- 没有自动化的前端构建流程

**目标**：
- 使用 esbuild 压缩所有 JS/CSS 文件（包括第三方库）
- 为每个文件添加内容哈希后缀（如 `custom.min.a1b2c3d4.css`）
- 自动更新 HTML 模板中的资源引用
- 在 Go 编译前执行前端构建

## 实施方案

### 1. 文件结构

```
web/
├── src/                         # 源文件目录（不参与构建）
│   ├── assets/                  # 前端资源源文件
│   │   ├── js/
│   │   │   └── inbound.js
│   │   └── css/
│   │       └── custom.css
│   └── html/                    # HTML 模板文件源
├── build/                       # 构建输出目录
│   ├── assets/                  # 压缩后的资源文件
│   │   ├── js/
│   │   │   └── inbound.min.a1b2c3d4.js
│   │   └── css/
│   │       └── custom.min.e5f6g7h8.css
│   ├── html/                    # 更新后的 HTML 模板
│   │   └── **/*.html            # 资源引用已更新为带哈希版本
│   └── manifest.json            # 资源映射清单
├── scripts/                     # 构建脚本
│   ├── build.js                 # 主构建脚本
│   └── update-html.js           # HTML 更新脚本
├── package.json                 # npm 依赖配置
├── esbuild.config.js            # esbuild 配置
└── esbuild.ignore               # 忽略文件列表（字体等）
```

### 2. 关键文件

#### 2.1 web/package.json

定义 npm 依赖和构建脚本：

```json
{
  "name": "3x-ui-fleet-hub-frontend",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "node scripts/build.js",
    "clean": "node scripts/build.js --clean"
  },
  "devDependencies": {
    "esbuild": "^0.20.0"
  }
}
```

#### 2.2 web/esbuild.config.js

esbuild 配置和入口点定义：

```javascript
import path from 'path';
import fs from 'fs';
import crypto from 'crypto';
import { fileURLToPath } from 'url';
import { dirname } from 'path';

// ES modules 中获取 __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// 生成内容哈希
function generateHash(content) {
  return crypto.createHash('sha512').update(content).digest('hex')
}

// 获取所有需要处理的 JS 文件
function getJSEntries() {
  const jsDir = path.join(__dirname, 'src/assets');
  const entries = {};

  function walkDir(dir, baseDir = '') {
    const files = fs.readdirSync(dir);
    for (const file of files) {
      const fullPath = path.join(dir, file);
      const stat = fs.statSync(fullPath);

      if (stat.isDirectory()) {
        walkDir(fullPath, path.join(baseDir, file));
      } else if (file.endsWith('.js')) {
        const relativePath = path.join(baseDir, file.replace('.js', ''));
        const key = relativePath.replace(/\\/g, '/');
        entries[key] = fullPath;
      }
    }
  }

  // 处理 js/ 目录
  const jsPath = path.join(jsDir, 'js');
  if (fs.existsSync(jsPath)) {
    walkDir(jsPath, 'js');
  }

  // 处理第三方库
  const vendors = [
    'vue/vue.min.js',
    'axios/axios.min.js',
    'moment/moment.min.js',
    'moment/moment-jalali.min.js',
    'qs/qs.min.js',
    'ant-design-vue/antd.min.js',
    'codemirror/codemirror.min.js',
    'codemirror/javascript.js',
    'codemirror/jshint.js',
    'codemirror/jsonlint.js',
    'codemirror/fold/foldcode.js',
    'codemirror/fold/foldgutter.js',
    'codemirror/fold/brace-fold.js',
    'codemirror/hint/javascript-hint.js',
    'codemirror/lint/lint.js',
    'codemirror/lint/javascript-lint.js',
    'otpauth/otpauth.umd.min.js',
    'qrcode/qrious2.min.js',
    'uri/URI.min.js',
    'persian-datepicker/persian-datepicker.min.js',
  ];

  for (const vendor of vendors) {
    const vendorPath = path.join(jsDir, vendor);
    if (fs.existsSync(vendorPath)) {
      const key = vendor.replace('.js', '').replace(/\\/g, '/');
      entries[key] = vendorPath;
    }
  }

  return entries;
}

// 获取所有需要处理的 CSS 文件
function getCSSEntries() {
  const cssDir = path.join(__dirname, 'src/assets');
  const entries = {};

  function walkDir(dir, baseDir = '') {
    const files = fs.readdirSync(dir);
    for (const file of files) {
      const fullPath = path.join(dir, file);
      const stat = fs.statSync(fullPath);

      if (stat.isDirectory()) {
        walkDir(fullPath, path.join(baseDir, file));
      } else if (file.endsWith('.css')) {
        const relativePath = path.join(baseDir, file.replace('.css', ''));
        const key = relativePath.replace(/\\/g, '/');
        entries[key] = fullPath;
      }
    }
  }

  // 处理所有 CSS 文件
  walkDir(cssDir, '');

  return entries;
}

// 获取不需要压缩的文件（字体等）
function getCopyFiles() {
  return [
    'Vazirmatn-UI-NL-Regular.woff2',
  ];
}

export default {
  generateHash,
  getJSEntries,
  getCSSEntries,
  getCopyFiles,
  paths: {
    root: path.join(__dirname),
    srcAssets: path.join(__dirname, 'src/assets'),
    srcHtml: path.join(__dirname, 'src/html'),
    buildAssets: path.join(__dirname, 'build/assets'),
    buildHtml: path.join(__dirname, 'build/html'),
    build: path.join(__dirname, 'build'),
  },
};
```

#### 2.3 web/scripts/build.js

主构建脚本：

```javascript
#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import esbuild from 'esbuild';
import config from '../esbuild.config.js';

const args = process.argv.slice(2);
const isClean = args.includes('--clean');

// 清理构建目录
function clean() {
  const dirsToClean = [config.paths.build];
  for (const dir of dirsToClean) {
    if (fs.existsSync(dir)) {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }
  console.log('✓ Cleaned build directories');
}

// 压缩 JS 文件
async function buildJS(manifest) {
  console.log('→ Building JS files...');

  const buildDir = config.paths.buildAssets;
  if (!fs.existsSync(buildDir)) {
    fs.mkdirSync(buildDir, { recursive: true });
  }

  const entries = config.getJSEntries();

  for (const [name, entry] of Object.entries(entries)) {
    try {
      const result = await esbuild.build({
        entryPoints: [entry],
        bundle: false,
        minify: true,
        outfile: path.join(buildDir, `${name}.min.js`),
        target: 'es2015',
        write: false,
      });

      const outputFiles = result.outputFiles;
      if (outputFiles && outputFiles.length > 0) {
        const content = outputFiles[0].contents;
        const hash = config.generateHash(content);
        const hashFileName = `${name}.min.${hash}.js`;
        const hashPath = path.join(buildDir, hashFileName);

        // 写入带哈希的文件
        const hashDir = path.dirname(hashPath);
        if (!fs.existsSync(hashDir)) {
          fs.mkdirSync(hashDir, { recursive: true });
        }
        fs.writeFileSync(hashPath, content);

        manifest[name] = {
          hash: hashFileName,
          size: content.length,
        };

        console.log(`  ✓ ${name}.js → ${hashFileName} (${(content.length / 1024).toFixed(1)} KB)`);
      }
    } catch (error) {
      console.error(`  ✗ Error building ${name}:`, error.message);
      throw error;
    }
  }

  console.log('✓ Built JS files');
}

// 压缩 CSS 文件
async function buildCSS(manifest) {
  console.log('→ Building CSS files...');

  const buildDir = config.paths.buildAssets;
  const entries = config.getCSSEntries();

  for (const [name, entry] of Object.entries(entries)) {
    try {
      const result = await esbuild.build({
        entryPoints: [entry],
        bundle: false,
        minify: true,
        outfile: path.join(buildDir, `${name}.min.css`),
        write: false,
      });

      const outputFiles = result.outputFiles;
      if (outputFiles && outputFiles.length > 0) {
        const content = outputFiles[0].contents;
        const hash = config.generateHash(content);
        const hashFileName = `${name}.min.${hash}.css`;
        const hashPath = path.join(buildDir, hashFileName);

        const hashDir = path.dirname(hashPath);
        if (!fs.existsSync(hashDir)) {
          fs.mkdirSync(hashDir, { recursive: true });
        }
        fs.writeFileSync(hashPath, content);

        manifest[name] = {
          hash: hashFileName,
          size: content.length,
        };

        console.log(`  ✓ ${name}.css → ${hashFileName} (${(content.length / 1024).toFixed(1)} KB)`);
      }
    } catch (error) {
      console.error(`  ✗ Error building ${name}:`, error.message);
    }
  }

  console.log('✓ Built CSS files');
}

// 复制不需要压缩的文件
function copyStaticFiles() {
  console.log('→ Copying static files...');

  const buildDir = config.paths.buildAssets;
  const files = config.getCopyFiles();

  for (const file of files) {
    const srcPath = path.join(config.paths.srcAssets, file);
    const destPath = path.join(buildDir, file);

    if (fs.existsSync(srcPath)) {
      const destDir = path.dirname(destPath);
      if (!fs.existsSync(destDir)) {
        fs.mkdirSync(destDir, { recursive: true });
      }
      fs.copyFileSync(srcPath, destPath);
      console.log(`  ✓ ${file}`);
    }
  }

  console.log('✓ Copied static files');
}

// 生成 manifest.json
function generateManifest(manifest) {
  console.log('→ Generating manifest.json...');

  const manifestPath = path.join(config.paths.build, 'manifest.json');
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));

  console.log('✓ Generated manifest.json');
  console.log(`  Total entries: ${Object.keys(manifest).length}`);
}

// 复制 src/html 到 build/html（保留原始结构）
function copyHtmlToBuild() {
  console.log('→ Copying HTML templates to build...');

  const srcHtmlDir = config.paths.srcHtml;
  const buildHtmlDir = config.paths.buildHtml;

  // 删除原 build/html 目录
  if (fs.existsSync(buildHtmlDir)) {
    fs.rmSync(buildHtmlDir, { recursive: true, force: true });
  }

  // 复制 src/html 到 build/html
  copyDir(srcHtmlDir, buildHtmlDir);

  console.log('✓ Copied HTML templates to build');
}

function copyDir(src, dest) {
  if (!fs.existsSync(dest)) {
    fs.mkdirSync(dest, { recursive: true });
  }

  const entries = fs.readdirSync(src, { withFileTypes: true });

  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);

    if (entry.isDirectory()) {
      copyDir(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

// 主构建流程
async function build() {
  console.log('\n' + '='.repeat(50));
  console.log('Building frontend (PRODUCTION)');
  console.log('='.repeat(50) + '\n');

  if (isClean) {
    clean();
    return;
  }

  const manifest = {};

  try {
    clean();
    await buildJS(manifest);
    await buildCSS(manifest);
    copyStaticFiles();
    generateManifest(manifest);
    copyHtmlToBuild();

    console.log('\n' + '='.repeat(50));
    console.log('✓ Build completed successfully!');
    console.log('='.repeat(50) + '\n');

  } catch (error) {
    console.error('\n✗ Build failed:', error);
    process.exit(1);
  }
}

build();
```

#### 2.4 web/scripts/update-html.js

HTML 模板更新脚本：

```javascript
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
```

#### 2.5 Makefile

在项目根目录创建 Makefile：

```makefile
.PHONY: help build clean frontend

help:
	@echo "Available targets:"
	@echo "  make build      - Build production release (frontend + Go binary)"
	@echo "  make frontend   - Build frontend only"
	@echo "  make clean      - Clean build artifacts"

# 安装前端依赖
web/node_modules:
	@echo "→ Installing frontend dependencies..."
	cd web && npm install

# 前端构建
frontend: web/node_modules
	@echo "→ Building frontend..."
	cd web && npm run build

# Go 构建
go-build:
	@echo "→ Building Go binary..."
	go build -o bin/3x-ui.exe main.go

# 完整构建
build: frontend go-build
	@echo "\n✓ Production build completed!"
	@echo "  Binary: bin/3x-ui.exe"

# 清理
clean:
	@echo "→ Cleaning build artifacts..."
	rm -rf web/build
	rm -rf web/node_modules
	rm -rf bin
```

### 3. 构建流程

1. **执行 `make build`** 或 **`make frontend`**
2. 清理 `web/build` 目录
3. 使用 esbuild 压缩所有 JS 文件（包括第三方库）→ 输出到 `web/build/assets/js/`
4. 使用 esbuild 压缩所有 CSS 文件 → 输出到 `web/build/assets/css/`
5. 复制静态文件（字体等）→ 输出到 `web/build/assets/`
6. 生成 `web/build/manifest.json` 资源映射
7. 复制 `web/src/html` 到 `web/build/html`
8. 更新 `web/build/html` 中所有 HTML 模板的资源引用路径
9. 编译 Go 程序（嵌入 `web/build` 目录的内容）

### 4. 关键实现细节

#### 4.1 文件哈希生成
使用 sha512 哈希前 8 位作为文件后缀：
```javascript
crypto.createHash('sha512').update(content).digest('hex')
```

#### 4.2 esbuild 配置
- `bundle: false` - 不打包，保持独立文件
- `minify: true` - 启用压缩
- `target: 'es2015'` - 兼容性目标
- `write: false` - 不直接写入，手动处理哈希

#### 4.3 HTML 更新策略
- 使用正则表达式匹配资源引用
- 保留 `{{ .base_path }}` 和 `{{ .cur_ver }}` 模板变量
- 只替换文件名部分，不改变路径结构

#### 4.4 与 Go 集成
- 需要更新 `web/web.go` 中的 embed 路径：
  - `//go:embed build/assets` - 嵌入压缩后的资源文件
  - `//go:embed build/html` - 嵌入更新后的 HTML 模板
- 或者保持原结构，在构建后将 `web/build` 内容复制到 `web/assets` 和 `web/html`
- 构建顺序：先前端，后 Go

### 5. 预期效果

| 指标 | 当前 | 压缩后 | 改善 |
|------|------|--------|------|
| JS 文件大小 | ~2.5MB | ~1.0MB | ~60% |
| CSS 文件大小 | ~520KB | ~200KB | ~62% |
| 总文件大小 | ~8.3MB | ~6.8MB | ~18% |
| 缓存策略 | 查询参数 | 内容哈希 | ✓ 永久缓存 |

### 6. 使用方法

```bash
# 首次使用，安装依赖
make build

# 后续构建
make build

# 仅构建前端
make frontend

# 清理构建产物
make clean
```

### 7. 故障排查

**问题**: esbuild 构建失败
- 检查文件语法：`node -c web/src/assets/js/file.js`
- 清理并重新构建：`make clean && make build`

**问题**: HTML 更新不生效
- 检查 `web/build/manifest.json` 是否生成
- 检查 `web/build/html` 目录是否存在
- 手动运行：`cd web && node scripts/update-html.js`

**问题**: Go 嵌入资源为空
- 检查构建输出：`ls -la web/build/`
- 检查资源文件：`ls -la web/build/assets/`
- 检查 HTML 文件：`ls -la web/build/html/`
- 确认构建成功完成

## 关键文件清单

实施此方案需要创建/修改的文件：

- **web/package.json** - npm 依赖配置（新建）
- **web/esbuild.config.js** - esbuild 配置（新建）
- **web/scripts/build.js** - 主构建脚本（新建）
- **web/scripts/update-html.js** - HTML 更新脚本（新建）
- **Makefile** - 构建流程集成（新建）

**源文件（不修改）**：
- **web/src/assets/** - 前端资源源文件（JS/CSS/字体等）
- **web/src/html/** - HTML 模板源文件

**构建输出（自动生成）**：
- **web/build/assets/** - 压缩后的资源文件（带内容哈希）
- **web/build/html/** - 更新后的 HTML 模板（资源引用已更新）
- **web/build/manifest.json** - 资源映射清单

**Go 集成（需要修改）**：
- **web/web.go** - 需要更新 embed 路径为 `build/assets` 和 `build/html`
- 或在 Makefile 中添加复制步骤，将 build 内容复制回 assets 和 html 目录
