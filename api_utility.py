import requests
import json
import logging

# 配置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class APIClient:
    """
    通用 API 请求工具类
    """
    
    @staticmethod
    def get(url, headers=None, params=None):
        """
        通用 GET 请求
        :param url: 请求地址
        :param headers: 请求头 (Dict)
        :param params: URL 参数 (Dict)
        :return: JSON 化的响应内容 (Dict)
        """
        try:
            logger.info(f"发送 GET 请求: {url}")
            response = requests.get(url, headers=headers, params=params, timeout=15)
            # 记录状态码
            logger.info(f"响应状态码: {response.status_code}")
            
            # 尝试返回 JSON
            return response.json()
        except requests.exceptions.JSONDecodeError:
            logger.warning("响应内容不是有效的 JSON 格式")
            return {"text": response.text, "status_code": response.status_code}
        except Exception as e:
            logger.error(f"GET 请求发生异常: {str(e)}")
            return {"error": str(e)}

    @staticmethod
    def post(url, headers=None, json_data=None, form_data=None):
        """
        通用 POST 请求
        :param url: 请求地址
        :param headers: 请求头 (Dict)
        :param json_data: JSON 格式的 Body (Dict)
        :param form_data: 表单格式的 Body (Dict)
        :return: JSON 化的响应内容 (Dict)
        """
        try:
            logger.info(f"发送 POST 请求: {url}")
            if json_data:
                response = requests.post(url, headers=json_data, json=json_data, timeout=15)
            else:
                response = requests.post(url, headers=form_data, data=form_data, timeout=15)
            
            logger.info(f"响应状态码: {response.status_code}")
            
            return response.json()
        except requests.exceptions.JSONDecodeError:
            logger.warning("响应内容不是有效的 JSON 格式")
            return {"text": response.text, "status_code": response.status_code}
        except Exception as e:
            logger.error(f"POST 请求发生异常: {str(e)}")
            return {"error": str(e)}

# 快捷调用方法
def get(url, headers=None, params=None):
    return APIClient.get(url, headers, params)

def post(url, headers=None, json_data=None, form_data=None):
    return APIClient.post(url, headers, json_data, form_data)

if __name__ == "__main__":
    # 测试范例
    # test_url = "https://httpbin.org/get"
    # result = get(test_url)
    # print("\n--- GET 测试结果 ---")
    # print(json.dumps(result, indent=2, ensure_ascii=False))
    
    test_post_url = "https://api.novelnovastory.com/login/anonymous"
    post_result = post(test_post_url, json_data={"app": "com.novelnova.readstory", "device-uuid": "BEACDA49-AFA1-4D2E-B32E-186ACDD00EDC", "User-Agent": "NovelNova/1.4.0 (iPhone; iOS 26.1)","Content-Type":"application/json"})
    print("\n--- POST 测试结果 ---")
    print(json.dumps(post_result, indent=2, ensure_ascii=False))
