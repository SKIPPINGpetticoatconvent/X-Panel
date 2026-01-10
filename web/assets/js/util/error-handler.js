/**
 * 证书错误处理工具
 * 处理 CERT_E 系列错误码的国际化显示
 */
class CertErrorHandler {
  /**
   * 获取证书错误消息
   * @param {string} errorCode - 错误码，如 "CERT_E001"
   * @param {string} fallbackMessage - 后备消息
   * @param {string} language - 语言代码，默认从浏览器获取
   * @returns {string} 用户友好的错误消息
   */
  static getCertErrorMessage(errorCode, fallbackMessage = "", language = null) {
    try {
      // 获取当前语言
      const currentLang = language || this.getCurrentLanguage();

      // 检查是否为证书错误码
      if (!errorCode || !errorCode.startsWith("CERT_E")) {
        return fallbackMessage || errorCode || "Unknown error";
      }

      // 从翻译系统中获取消息
      const translatedMessage = this.getTranslatedMessage(errorCode, currentLang);

      // 如果找到翻译，返回翻译后的消息
      if (translatedMessage) {
        return translatedMessage;
      }

      // 如果没有找到翻译，返回英文默认消息
      const defaultMessage = this.getDefaultEnglishMessage(errorCode);
      if (defaultMessage) {
        return defaultMessage;
      }

      // 最后的后备方案
      return fallbackMessage || `Certificate Error: ${errorCode}`;
    } catch (error) {
      console.error("Error in getCertErrorMessage:", error);
      return fallbackMessage || errorCode || "Unknown error";
    }
  }

  /**
   * 从翻译系统中获取消息
   * @param {string} errorCode - 错误码
   * @param {string} language - 语言代码
   * @returns {string|null} 翻译后的消息或null
   */
  static getTranslatedMessage(errorCode, language) {
    // 这里假设有一个全局的翻译函数或对象
    // 在实际实现中，需要根据前端框架的具体翻译机制来调整

    // 尝试从 Vue i18n 或其他翻译系统获取
    if (typeof window !== "undefined" && window.Vue && window.Vue.prototype) {
      try {
        const i18n = window.Vue.prototype.$t;
        if (i18n && typeof i18n === "function") {
          return i18n(`cert_errors.${errorCode}`);
        }
      } catch (e) {
        // 忽略错误，继续尝试其他方法
      }
    }

    // 如果没有全局翻译系统，返回null
    return null;
  }

  /**
   * 获取默认英文消息
   * @param {string} errorCode - 错误码
   * @returns {string|null} 默认英文消息或null
   */
  static getDefaultEnglishMessage(errorCode) {
    const defaultMessages = {
      "CERT_E001": "Port 80 is occupied by another process",
      "CERT_E002": "Port 80 is occupied by external process (e.g., Nginx)",
      "CERT_E003": "CA server timeout",
      "CERT_E004": "CA server refused the request",
      "CERT_E005": "DNS resolution failed",
      "CERT_E006": "Certificate has expired",
      "CERT_E007": "Certificate renewal failed",
      "CERT_E008": "Xray core reload failed",
      "CERT_E009": "Fallback mode activated",
      "CERT_E010": "Permission denied",
    };

    return defaultMessages[errorCode] || null;
  }

  /**
   * 获取当前语言
   * @returns {string} 语言代码
   */
  static getCurrentLanguage() {
    try {
      // 尝试从 Cookie 获取语言设置
      const langCookie = CookieManager.getCookie("lang");
      if (langCookie) {
        return langCookie;
      }

      // 从浏览器 navigator 获取
      if (navigator && navigator.language) {
        return navigator.language;
      }

      // 默认中文
      return "zh-CN";
    } catch (error) {
      console.error("Error getting current language:", error);
      return "zh-CN";
    }
  }

  /**
   * 处理API响应中的证书错误
   * @param {Object} response - API响应对象
   * @returns {Object} 处理后的响应对象
   */
  static processApiResponse(response) {
    if (!response || typeof response !== "object") {
      return response;
    }

    // 检查是否包含证书错误
    if (response.error && response.error.code && response.error.code.startsWith("CERT_E")) {
      const userMessage = this.getCertErrorMessage(response.error.code, response.error.message);
      return {
        ...response,
        error: {
          ...response.error,
          userMessage: userMessage,
        },
      };
    }

    return response;
  }

  /**
   * 显示证书错误提示
   * @param {string} errorCode - 错误码
   * @param {string} details - 错误详情
   * @param {string} type - 提示类型 ('error', 'warning', 'info')
   */
  static showCertErrorToast(errorCode, details = "", type = "error") {
    const message = this.getCertErrorMessage(errorCode);

    // 使用 Vue 的消息提示
    if (window.Vue && window.Vue.prototype && window.Vue.prototype.$message) {
      const toastType = type === "error" ? "error" : type === "warning" ? "warning" : "info";
      window.Vue.prototype.$message[toastType](message);

      // 如果有详情，显示在控制台
      if (details) {
        console.warn(`Certificate error details [${errorCode}]:`, details);
      }
    } else {
      // 后备方案：使用原生 alert
      alert(message);
    }
  }
}

// 导出为全局对象
if (typeof window !== "undefined") {
  window.CertErrorHandler = CertErrorHandler;
}
