import fs from 'fs';
import path from 'path';

// 从环境变量读取配置，提供默认值进行单侧兼容
const EMAIL = process.env.EMAIL || "test_super_001@shortswave.com";
const PASSWORD = process.env.PASSWORD || "test123456";
const LOGIN_URL = process.env.LOGIN_URL || "http://35.225.224.94:8080/api/pwd_login";
const DRAMA_LIST_URL = process.env.DRAMA_LIST_URL || "http://35.225.224.94:8080/api/management/drama/all_online_ids";

// 定义最终落盘的存储文件路径 (锁定在项目根目录)
const OUTPUT_FILE = path.join(import.meta.dirname, '..', 'drama_info.json');

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

  const loginRes = await fetch(LOGIN_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Connection': 'close'
    },
    body: JSON.stringify(loginPayload)
  });

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
  const dramaRes = await fetch(DRAMA_LIST_URL, {
    method: 'GET',
    headers: {
      // 通过 Cookie 附带身份凭证进行鉴权
      'Cookie': `x-token=${xToken}`,
      'Connection': 'close'
    }
  });

  if (!dramaRes.ok) {
    const errorText = await dramaRes.text();
    console.error('❌ 获取剧集列表失败, HTTP 状态码:', dramaRes.status);
    console.error('❌ 响应内容:', errorText);
    process.exit(1);
  }

  const dramaDataText = await dramaRes.text();
  let dramaIds = [];
  try {
     // 兼容性识别：如果返回的是 CSV 格式 (含有 _id,int_id 表头)
     if (dramaDataText.includes('_id') || dramaDataText.includes(',')) {
        console.log('📝 检测到 CSV 格式，正在提取 hex _id 列...');
        const lines = dramaDataText.split(/[\n\r]+/).filter(l => l.trim() !== '');
        // 跳过表头，提取每行第一列
        dramaIds = lines.slice(1)
                      .map(line => line.split(',')[0].trim())
                      .filter(id => /^[0-9a-fA-F]{24}$/.test(id)); // 只保留 24 位 hex ID
     } else {
        // 兜底逻辑：尝试作为 JSON 解析
        const rawJson = JSON.parse(dramaDataText);
        dramaIds = Array.isArray(rawJson) ? rawJson : [];
     }
  } catch (e) {
     console.log('⚠️ 数据解析异常，尝试正则提取所有 hex ID...');
     dramaIds = dramaDataText.match(/[0-9a-fA-F]{24}/g) || [];
  }
  
  console.log(`✅ 成功获取全部剧集信息，有效 Hex _id 条数: ${dramaIds.length}`);

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
    apiBase: DRAMA_LIST_URL.split('/api/')[0],
    // 只存我们需要的 int_id 数组即可
    dramaList: dramaIds 
  };

  fs.writeFileSync(OUTPUT_FILE, JSON.stringify(targetOutput, null, 2), 'utf8');
  console.log(`🎉 任务完成！所有核心准备数据已封装备用: [${OUTPUT_FILE}]`);
}

main().catch(err => {
  console.error('🔥 脚本执行发生致命异常:', err);
  process.exit(1);
});
