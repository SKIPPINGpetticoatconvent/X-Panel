axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

// 【新增】: API请求超时配置
const REQUEST_TIMEOUTS = {
    GET: 10000,        // GET请求10秒
    POST: 15000,       // POST请求15秒  
    PUT: 20000,        // PUT请求20秒
    DELETE: 10000,     // DELETE请求10秒
    UPLOAD: 60000,     // 文件上传60秒
    DEFAULT: 15000     // 默认15秒
};

// 【新增】: 根据请求类型和路径获取超时时间
function getRequestTimeout(config) {
    const method = config.method.toUpperCase();
    const url = config.url || '';
    
    // 文件上传相关请求
    if (url.includes('/upload') || url.includes('/file') || config.headers?.['Content-Type']?.includes('multipart/form-data')) {
        return REQUEST_TIMEOUTS.UPLOAD;
    }
    
    // 根据HTTP方法返回对应超时时间
    switch (method) {
        case 'GET':
            return REQUEST_TIMEOUTS.GET;
        case 'POST':
            return REQUEST_TIMEOUTS.POST;
        case 'PUT':
            return REQUEST_TIMEOUTS.PUT;
        case 'DELETE':
            return REQUEST_TIMEOUTS.DELETE;
        default:
            return REQUEST_TIMEOUTS.DEFAULT;
    }
}

// 【新增】: 格式化超时错误消息
function formatTimeoutMessage(config, timeout) {
    const method = config.method.toUpperCase();
    const url = config.url || 'Unknown URL';
    
    let message = `请求超时（${timeout/1000}秒）`;
    
    if (method !== 'GET') {
        message += ` - ${method} ${url}`;
    }
    
    return message;
}

// 【新增】: 显示用户友好的超时错误提示
function showTimeoutError(config, timeout) {
    if (Vue && Vue.prototype && Vue.prototype.$message) {
        const message = formatTimeoutMessage(config, timeout);
        const timeoutMs = timeout / 1000;
        
        Vue.prototype.$error({
            title: '⏰ 请求超时',
            content: `${message}\n\n可能原因：\n• 网络连接不稳定\n• 服务器响应较慢\n• 请求数据量过大\n\n建议：\n• 检查网络连接\n• 稍后重试\n• 如问题持续，请联系管理员`,
            okText: '重试',
            onOk: () => {
                // 重新发送请求
                const originalConfig = { ...config };
                originalConfig.timeout = getRequestTimeout(originalConfig);
                return axios(originalConfig);
            },
            cancelText: '取消',
            duration: 10
        });
    } else {
        console.warn('请求超时:', formatTimeoutMessage(config, timeout));
    }
}

axios.interceptors.request.use(
    (config) => {
        // 【增强】: 为每个请求设置超时时间
        const timeout = getRequestTimeout(config);
        config.timeout = timeout;
        
        // 设置请求开始时间（用于性能监控）
        config.metadata = config.metadata || {};
        config.metadata.startTime = Date.now();
        
        if (config.data instanceof FormData) {
            config.headers['Content-Type'] = 'multipart/form-data';
        } else {
            config.data = Qs.stringify(config.data, {
                arrayFormat: 'repeat',
            });
        }
        
        console.log(`🚀 发起请求: ${config.method.toUpperCase()} ${config.url} (超时: ${timeout/1000}秒)`);
        return config;
    },
    (error) => {
        console.error('❌ 请求拦截器错误:', error);
        return Promise.reject(error);
    }
);

axios.interceptors.response.use(
    (response) => {
        // 【新增】: 计算请求耗时
        if (response.config?.metadata?.startTime) {
            const duration = Date.now() - response.config.metadata.startTime;
            console.log(`✅ 请求完成: ${response.config.method.toUpperCase()} ${response.config.url} (耗时: ${duration}ms)`);
            
            // 【新增】: 性能监控 - 记录慢请求
            if (duration > 5000) {
                console.warn(`🐌 慢请求检测: ${duration}ms - ${response.config.method.toUpperCase()} ${response.config.url}`);
            }
        }
        
        return response;
    },
    (error) => {
        // 【增强】: 改进的响应错误处理
        const config = error.config;
        const response = error.response;
        
        if (config?.metadata?.startTime) {
            const duration = Date.now() - config.metadata.startTime;
            console.log(`❌ 请求失败: ${config.method.toUpperCase()} ${config.url} (耗时: ${duration}ms)`);
        }
        
        // 【新增】: 处理超时错误
        if (error.code === 'ECONNABORTED' || error.message.includes('timeout')) {
            const timeout = config?.timeout || REQUEST_TIMEOUTS.DEFAULT;
            console.error(`⏰ 请求超时: ${formatTimeoutMessage(config, timeout)}`);
            showTimeoutError(config, timeout);
            
            return Promise.reject(new Error(formatTimeoutMessage(config, timeout)));
        }
        
        // 【新增】: 处理网络错误
        if (!error.response) {
            const message = error.message || '网络连接失败';
            console.error('🌐 网络错误:', message);
            
            if (Vue && Vue.prototype && Vue.prototype.$message) {
                Vue.prototype.$error({
                    title: '🌐 网络错误',
                    content: `${message}\n\n请检查：\n• 网络连接状态\n• 服务器是否正常运行\n• 防火墙设置`,
                    okText: '重试',
                    onOk: () => {
                        // 重新发送请求
                        const originalConfig = { ...config };
                        originalConfig.timeout = getRequestTimeout(originalConfig);
                        return axios(originalConfig);
                    },
                    cancelText: '取消',
                    duration: 8
                });
            }
            
            return Promise.reject(new Error(message));
        }
        
        // 原有的状态码处理
        if (error.response) {
            const statusCode = error.response.status;
            const statusText = error.response.statusText;
            const errorData = error.response.data;
            
            console.error(`📊 HTTP错误 ${statusCode}: ${statusText}`, errorData);
            
            // Check the status code
            if (statusCode === 401) { // Unauthorized
                console.warn('🔒 认证失败，正在重定向到登录页面...');
                return window.location.reload();
            }
            
            // 【新增】: 为常见错误提供用户友好的消息
            let userMessage = `请求失败 (${statusCode})`;
            
            if (statusCode === 403) {
                userMessage = '权限不足，无法访问此资源';
            } else if (statusCode === 404) {
                userMessage = '请求的资源不存在';
            } else if (statusCode >= 500) {
                userMessage = '服务器内部错误，请稍后重试';
            } else if (errorData && errorData.msg) {
                userMessage = errorData.msg;
            }
            
            if (Vue && Vue.prototype && Vue.prototype.$message && statusCode >= 500) {
                Vue.prototype.$error({
                    title: '服务器错误',
                    content: userMessage,
                    duration: 6
                });
            }
        }
        
        return Promise.reject(error);
    }
);

// 【新增】: 全局错误处理函数
window.handleGlobalError = function(error, context = '') {
    const errorInfo = {
        timestamp: new Date().toISOString(),
        message: error.message || 'Unknown error',
        stack: error.stack,
        context: context,
        userAgent: navigator.userAgent,
        url: window.location.href
    };
    
    console.error('💥 全局错误:', errorInfo);
    
    // 可以在这里添加错误上报逻辑
    // 例如发送到错误监控服务
};

// 【新增】: 导出超时配置供其他模块使用
window.REQUEST_TIMEOUTS = REQUEST_TIMEOUTS;
window.getRequestTimeout = getRequestTimeout;