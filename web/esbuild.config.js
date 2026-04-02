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
  return crypto.createHash('sha512').update(content).digest('hex');
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
        const relativePath = path.join(baseDir, file);
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
      const key = vendor.replace(/\\/g, '/'); // 保留完整的文件名包括 .js
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
        const relativePath = path.join(baseDir, file); // 保留完整的文件名包括 .css
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
