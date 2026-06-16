import fs from 'fs';
import path from 'path';

// 从环境变量读取配置，提供默认值进行单侧兼容
const EMAIL = process.env.EMAIL || "test_super_001@shortswave.com";
const PASSWORD = process.env.PASSWORD || "test123456";
const LOGIN_URL = process.env.LOGIN_URL || "http://35.225.224.94:8080/api/pwd_login";
const DRAMA_LIST_URL = process.env.DRAMA_LIST_URL || "http://35.225.224.94:8080/api/management/drama/all_online_ids";
const FETCH_RETRY_ATTEMPTS = Number(process.env.PREPARE_FETCH_RETRY_ATTEMPTS || 3);
const DRAMA_META_PAGE_SIZE = Number(process.env.DRAMA_META_PAGE_SIZE || 200);

// 定义最终落盘的存储文件路径 (锁定在项目根目录)
const OUTPUT_FILE = path.join(import.meta.dirname, '..', 'drama_info.json');

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

async function fetchWithRetry(url, options, label) {
  let lastError;
  for (let attempt = 1; attempt <= FETCH_RETRY_ATTEMPTS; attempt++) {
    try {
      const response = await fetch(url, options);
      if (response.ok || attempt >= FETCH_RETRY_ATTEMPTS || !shouldRetryStatus(response.status)) {
        return response;
      }
      const bodyText = await safeReadText(response);
      console.warn(`⚠️ ${label} 第 ${attempt}/${FETCH_RETRY_ATTEMPTS} 次请求失败，HTTP ${response.status}，准备重试。响应: ${bodyText}`);
    } catch (error) {
      lastError = error;
      if (attempt >= FETCH_RETRY_ATTEMPTS) {
        throw error;
      }
      console.warn(`⚠️ ${label} 第 ${attempt}/${FETCH_RETRY_ATTEMPTS} 次请求异常，准备重试: ${formatFetchError(error)}`);
    }
    await sleep(500 * attempt);
  }
  throw lastError || new Error(`${label} 请求失败`);
}

function shouldRetryStatus(status) {
  return status === 408 || status === 429 || status >= 500;
}

async function safeReadText(response) {
  try {
    return await response.clone().text();
  } catch (error) {
    return `读取响应失败: ${formatFetchError(error)}`;
  }
}

function formatFetchError(error) {
  const cause = error?.cause;
  const parts = [error?.message || String(error)];
  if (cause?.code) parts.push(`code=${cause.code}`);
  if (cause?.host) parts.push(`host=${cause.host}`);
  if (cause?.port) parts.push(`port=${cause.port}`);
  return parts.join(', ');
}

async function main() {
  console.log('🚀 开始执行前置数据准备任务...');
  console.log(`📡 目标环境: ${DRAMA_LIST_URL.includes('admin') ? '正式服' : '测试服'}`);
  console.log(`👤 目标账户: ${EMAIL}`);
  console.log(`📝 存储位置: ${OUTPUT_FILE} (执行强制覆盖并刷新)`);
  
  // -------------------------------------------------------------
  // Step 1: 模拟登录获取 x-token
  // -------------------------------------------------------------
  console.log(`\n🎫 Step 1: 请求登录接口 [${LOGIN_URL}]`);
  const loginPayload = {
    app: "com.shorts.wave.drama",
    email: EMAIL,
    password: PASSWORD,
  };

  const loginRes = await fetchWithRetry(LOGIN_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Connection': 'close'
    },
    body: JSON.stringify(loginPayload)
  }, '登录接口');

  if (!loginRes.ok) {
    const errorText = await loginRes.text();
    console.error('❌ 登录失败, HTTP 状态码:', loginRes.status);
    console.error('❌ 响应内容:', errorText);
    process.exit(1);
  }

  // 从响应头解析 Set-Cookie
  const setCookieHeader = loginRes.headers.get('set-cookie');
  if (!setCookieHeader) {
    console.error('❌ 登录成功但未在此请求中找到 Set-Cookie Header！');
    process.exit(1);
  }

  // 粗略解析出 x-token 的值
  // 格式例如: x-token=1234567890; Path=/; HttpOnly
  const cookies = setCookieHeader.split(','); // node fetch 多个 cookie 时通过 `,` 或者直接数组挂载
  let xToken = '';
  // 如果服务端遵循标准返回多个 set-cookie 头，则 node api 获取 get 会拼成 "cookie1=val1; Path=/, cookie2=val2; Path=/"
  const match = setCookieHeader.match(/x-token=([^;]+)/);
  if (match && match[1]) {
    xToken = match[1];
    console.log(`✅ 成功获取 x-token: ${xToken.substring(0, 15)}...`);
  } else {
    console.error('❌ 在 Cookie 中未找到 x-token 字段:', setCookieHeader);
    process.exit(1);
  }

  // -------------------------------------------------------------
  // Step 2: 携带 Token 获取在线剧集 ID
  // -------------------------------------------------------------
  console.log(`\n🎬 Step 2: 携带 Token 请求剧集列表 [${DRAMA_LIST_URL}]`);
  const dramaRes = await fetchWithRetry(DRAMA_LIST_URL, {
    method: 'GET',
    headers: {
      // 通过 Cookie 附带身份凭证进行鉴权
      'Cookie': `x-token=${xToken}`,
      'Connection': 'close'
    }
  }, '剧集列表接口');

  if (!dramaRes.ok) {
    const errorText = await dramaRes.text();
    console.error('❌ 获取剧集列表失败, HTTP 状态码:', dramaRes.status);
    console.error('❌ 响应内容:', errorText);
    process.exit(1);
  }

  const dramaDataText = await dramaRes.text();
  let dramaIds = [];
  const dramaMeta = {};
  try {
     // 兼容性识别：如果返回的是 CSV 格式 (含有 _id,int_id 表头)
     if (dramaDataText.includes('_id') || dramaDataText.includes(',')) {
        console.log('📝 检测到 CSV 格式，正在提取 hex _id 列与剧集元信息...');
        const lines = dramaDataText.split(/[\n\r]+/).filter(l => l.trim() !== '');
        const headers = parseCSVLine(lines[0]).map(value => value.trim());
        const idIndex = findHeaderIndex(headers, ['_id', 'id', 'drama_id']);
        dramaIds = lines.slice(1).map(line => {
          const row = parseCSVLine(line);
          const id = (row[idIndex >= 0 ? idIndex : 0] || '').trim();
          if (/^[0-9a-fA-F]{24}$/.test(id)) {
            dramaMeta[id] = buildDramaMetaFromRecord(headers, row);
            return id;
          }
          return '';
        }).filter(Boolean);
     } else {
        // 兜底逻辑：尝试作为 JSON 解析
        const rawJson = JSON.parse(dramaDataText);
        const rawList = Array.isArray(rawJson) ? rawJson : [];
        dramaIds = rawList.map(item => {
          if (typeof item === 'string') return item;
          const id = String(item?._id || item?.id || item?.drama_id || '').trim();
          if (/^[0-9a-fA-F]{24}$/.test(id)) {
            dramaMeta[id] = normalizeDramaMeta({
              int_id: item?.int_id,
              title: item?.title,
              cn_title: item?.cn_title,
              cn_name: item?.cn_name
            });
            return id;
          }
          return '';
        }).filter(Boolean);
     }
  } catch (e) {
     console.log('⚠️ 数据解析异常，尝试正则提取所有 hex ID...');
     dramaIds = dramaDataText.match(/[0-9a-fA-F]{24}/g) || [];
  }
  
  console.log(`✅ 成功获取全部剧集信息，有效 Hex _id 条数: ${dramaIds.length}`);

  // -------------------------------------------------------------
  // Step 2.5: 补全剧集标题元信息，供 K6 报告和飞书总结直接复用
  // -------------------------------------------------------------
  const apiBase = DRAMA_LIST_URL.split('/api/')[0];
  console.log(`\n🏷️ Step 2.5: 补全剧集 title / cn_name 元信息`);
  const extraMeta = await fetchDramaMetadata(apiBase, xToken);
  let enrichedCount = 0;
  dramaIds.forEach(dramaId => {
    const merged = mergeDramaMeta(dramaMeta[dramaId], extraMeta[dramaId]);
    dramaMeta[dramaId] = merged;
    if (hasReadableDramaMeta(merged)) {
      enrichedCount++;
    }
  });
  console.log(`✅ 已补全剧集标题元信息: ${enrichedCount}/${dramaIds.length}`);

  // -------------------------------------------------------------
  // Step 3: 格式化包裹并持久化落盘
  // -------------------------------------------------------------
  console.log(`\n💾 Step 3: 持久化运行数据到本地 JSON 文件`);
  const targetOutput = {
    _meta: {
      generatedAt: new Date().toISOString(),
      source: "TestCenter Auto Prepare Script",
      email: EMAIL
    },
    auth: {
      x_token: xToken
    },
    // 将 API 基础路径透传给 K6 脚本，避免硬编码
    apiBase,
    dramaList: dramaIds,
    dramaMeta
  };

  fs.writeFileSync(OUTPUT_FILE, JSON.stringify(targetOutput, null, 2), 'utf8');
  console.log(`🎉 任务完成！所有核心准备数据已封装备用: [${OUTPUT_FILE}]`);
}

main().catch(err => {
  console.error('🔥 脚本执行发生致命异常:', err);
  process.exit(1);
});

function parseCSVLine(line) {
  const result = [];
  let current = '';
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const char = line[i];
    const next = line[i + 1];
    if (char === '"' && inQuotes && next === '"') {
      current += '"';
      i++;
      continue;
    }
    if (char === '"') {
      inQuotes = !inQuotes;
      continue;
    }
    if (char === ',' && !inQuotes) {
      result.push(current);
      current = '';
      continue;
    }
    current += char;
  }
  result.push(current);
  return result;
}

function findHeaderIndex(headers, candidates) {
  const normalized = headers.map(value => value.toLowerCase());
  return candidates.map(value => normalized.indexOf(value.toLowerCase())).find(index => index >= 0) ?? -1;
}

function buildDramaMetaFromRecord(headers, row) {
  const record = {};
  headers.forEach((header, index) => {
    record[header] = row[index];
  });
  return normalizeDramaMeta({
    int_id: firstRecordValue(record, ['int_id', 'intId']),
    title: firstRecordValue(record, ['title', 'name']),
    cn_title: firstRecordValue(record, ['cn_title', 'cnTitle', 'cn_name', 'cnName', '中文标题', '中文名']),
    cn_name: firstRecordValue(record, ['cn_name', 'cnName', 'cn_title', 'cnTitle', '中文名', '中文标题'])
  });
}

function firstRecordValue(record, keys) {
  for (const key of keys) {
    if (record[key] !== undefined && String(record[key]).trim() !== '') {
      return record[key];
    }
  }
  return '';
}

function normalizeDramaMeta(meta) {
  const cnTitle = String(meta.cn_title ?? meta.cnTitle ?? meta.cn_name ?? meta.cnName ?? '').trim();
  return {
    int_id: String(meta.int_id ?? meta.intId ?? '').trim(),
    title: String(meta.title ?? meta.name ?? '').trim(),
    cn_title: cnTitle,
    cn_name: String(meta.cn_name ?? meta.cnName ?? cnTitle).trim()
  };
}

async function fetchDramaMetadata(apiBase, xToken) {
  const metadata = {};
  const base = String(apiBase || '').replace(/\/+$/, '');
  if (!base || !xToken) {
    return metadata;
  }

  let page = 1;
  let total = 0;
  while (true) {
    const url = `${base}/api/management/drama/list?online=1&page=${page}&page_size=${DRAMA_META_PAGE_SIZE}`;
    let response;
    try {
      response = await fetchWithRetry(url, {
        method: 'GET',
        headers: {
          'Cookie': `x-token=${xToken}`,
          'Connection': 'close'
        }
      }, `剧集元信息接口 page=${page}`);
    } catch (error) {
      console.warn(`⚠️ 剧集元信息接口 page=${page} 请求异常，保留已获取标题并继续主检查: ${formatFetchError(error)}`);
      break;
    }

    if (!response.ok) {
      const errorText = await response.text();
      console.warn(`⚠️ 剧集元信息接口返回 HTTP ${response.status}，跳过标题补全。响应: ${errorText}`);
      break;
    }

    const payload = await response.json();
    const list = Array.isArray(payload?.data) ? payload.data : [];
    total = Number(payload?.total || total || 0);
    list.forEach(item => {
      const id = String(item?._id || item?.id || item?.drama_id || '').trim();
      if (/^[0-9a-fA-F]{24}$/.test(id)) {
        metadata[id] = normalizeDramaMeta(item);
      }
    });

    if (list.length === 0 || (total > 0 && page * DRAMA_META_PAGE_SIZE >= total)) {
      break;
    }
    page++;
  }

  return metadata;
}

function mergeDramaMeta(primary, secondary) {
  const left = normalizeDramaMeta(primary || {});
  const right = normalizeDramaMeta(secondary || {});
  return normalizeDramaMeta({
    int_id: left.int_id || right.int_id,
    title: left.title || right.title,
    cn_title: left.cn_title || right.cn_title,
    cn_name: left.cn_name || right.cn_name
  });
}

function hasReadableDramaMeta(meta) {
  return Boolean(String(meta?.title || '').trim() || String(meta?.cn_title || meta?.cn_name || '').trim());
}
