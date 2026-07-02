import http from 'k6/http';
import { check } from 'k6';
import { htmlReport } from "./vendor/k6-reporter/bundle.js";

// 全局配置参数 (优先从 Go 引擎注入的环境变量 __ENV 读取，以便后续平台化配置)
const EMAIL = __ENV.EMAIL || "";
const PASSWORD = __ENV.PASSWORD || "";
const LOGIN_URL = __ENV.LOGIN_URL || "http://35.225.224.94:8080/api/pwd_login";

// 基础执行配置
export const options = {
    // 接口测试初始化验证阶段，单次串行执行即可验证目标逻辑
    vus: 1,
    iterations: 1,
};

/**
 * setup() 为 K6 顶层生命周期，整个集群执行仅跑一次。
 * 完美契合使用场景：在这生成公共的 Cookie / Auth Token 进而返回分发给 VUS 共享。
 */
export function setup() {
    console.log(`[🚀] 发起登录鉴权请求，目标账户: ${EMAIL}...`);
    
    // 构造原 python 代码中的登录 payload
    const loginData = JSON.stringify({
        app: "com.shorts.wave.drama",
        email: EMAIL,
        password: PASSWORD,
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
            'Connection': 'close'
        },
        timeout: 10000,   // 对标 python timeout(5,10)
        redirects: 0      // 对标 python allow_redirects=False
    };

    const res = http.post(LOGIN_URL, loginData, params);
    
    let token = "";
    
    // 验证逻辑
    const loginCheck = check(res, {
        'Login HTTP Code: 200': (r) => r.status === 200,
        'Has x-token Cookie': (r) => {
            if (r.cookies && r.cookies['x-token']) {
                // k6 会将同名 cookie 组成数组
                const cookieArr = r.cookies['x-token'];
                if (cookieArr.length > 0) {
                    token = cookieArr[0].value;
                    return true;
                }
            }
            return false;
        }
    });

    if (loginCheck) {
        console.log(`[✅] 登录成功! 截获到 Token: ${token.substring(0, 15)}...`);
    } else {
        console.error(`[❌] 登录失败! Status: ${res.status}`);
        console.error(`[❌] Body info: ${res.body}`);
    }

    // 将截获到的 token 封装并 return。
    // K6 引擎随后会自动将该对象注入到内部的并发 default function 的入参(data)中共享
    return { token: token };
}

/**
 * 并发虚拟用户(VUS)主生命周期阶段
 */
export default function (data) {
    if (!data.token) {
        console.warn("[⚠️] 致命中断：未接收到上下文传递的有效 Auth Token，跳过后续业务测试。");
        return;
    }
    
    console.log(`[VUS] 成功接收全局 Token [${data.token.substring(0, 10)}]，准备启动下游业务接口并发链...`);
    
    // 下一阶：利用提取的 Token，在此处组装 Bearer 或 Cookie 头，针对目标业务接口发起连续并发请求
}

/**
 * 钩子销毁阶段：生成脱机版 HTML 仪表盘报表
 */
export function handleSummary(data) {
    return {
        // 输出到 Go 服务端开放的静态路由文件夹，以使得 Vue 前端也能直接获取这套详细数据
        "report/api_report/api_login_summary.html": htmlReport(data),
    };
}
