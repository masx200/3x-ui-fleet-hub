#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import esbuild from 'esbuild';
import { execSync } from 'child_process';
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
        outfile: path.join(buildDir, name),
        target: 'es2015',
        write: false,
      });

      const outputFiles = result.outputFiles;
      if (outputFiles && outputFiles.length > 0) {
        const content = outputFiles[0].contents;
        const hash = config.generateHash(content);

        // 移除原始扩展名，添加哈希，然后重新添加扩展名
        const baseName = name.replace(/\.js$/, '');
        const hashFileName = `${baseName}.${hash}.js`;
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

        console.log(`  ✓ ${name} → ${hashFileName} (${(content.length / 1024).toFixed(1)} KB)`);
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
        outfile: path.join(buildDir, name),
        write: false,
      });

      const outputFiles = result.outputFiles;
      if (outputFiles && outputFiles.length > 0) {
        const content = outputFiles[0].contents;
        const hash = config.generateHash(content);

        // 移除原始扩展名，添加哈希，然后重新添加扩展名
        const baseName = name.replace(/\.css$/, '');
        const hashFileName = `${baseName}.${hash}.css`;
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

        console.log(`  ✓ ${name} → ${hashFileName} (${(content.length / 1024).toFixed(1)} KB)`);
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

// 更新 HTML 模板中的资源引用
function updateHTMLReferences() {
  console.log('→ Updating HTML template references...');

  try {
    execSync('node scripts/update-html.js', {
      cwd: config.paths.root,
      stdio: 'inherit'
    });
    console.log('✓ Updated HTML template references');
  } catch (error) {
    console.error('✗ Failed to update HTML references:', error.message);
    throw error;
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
    updateHTMLReferences();

    console.log('\n' + '='.repeat(50));
    console.log('✓ Build completed successfully!');
    console.log('='.repeat(50) + '\n');

  } catch (error) {
    console.error('\n✗ Build failed:', error);
    process.exit(1);
  }
}

build();
