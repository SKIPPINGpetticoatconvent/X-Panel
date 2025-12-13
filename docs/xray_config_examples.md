# Xray 配置示例集合

本文档整理了从 XTLS/Xray-examples 仓库获取的 Xray 核心协议配置示例，包含 VLESS、VMess、Trojan 和 Shadowsocks 协议的完整配置。

## 目录
- [VLESS 协议配置](#vless-协议配置)
- [VMess 协议配置](#vmess-协议配置)
- [Trojan 协议配置](#trojan-协议配置)
- [Shadowsocks 协议配置](#shadowsocks-协议配置)

---

## VLESS 协议配置

### 1. VLESS-TCP-XTLS-Vision 完整配置

#### 服务器端配置
```jsonc
{
    "log": {
        "loglevel": "warning"
    },
    "routing": {
        "domainStrategy": "IPIfNonMatch",
        "rules": [
            {
                "ip": ["geoip:cn"],
                "outboundTag": "block"
            }
        ]
    },
    "inbounds": [
        {
            "listen": "0.0.0.0",
            "port": 443,
            "protocol": "vless",
            "settings": {
                "clients": [
                    {
                        "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",  // Generate with: xray uuid
                        "flow": "xtls-rprx-vision"
                    }
                ],
                "decryption": "none",
                "fallbacks": [
                    {
                        "dest": "8001",
                        "xver": 1
                    },
                    {
                        "alpn": "h2",
                        "dest": "8002",
                        "xver": 1
                    }
                ]
            },
            "streamSettings": {
                "network": "tcp",
                "security": "tls",
                "tlsSettings": {
                    "rejectUnknownSni": true,
                    "minVersion": "1.2",
                    "certificates": [
                        {
                            "ocspStapling": 3600,
                            "certificateFile": "/etc/ssl/private/fullchain.cer",
                            "keyFile": "/etc/ssl/private/private.key"
                        }
                    ]
                }
            },
            "sniffing": {
                "enabled": true,
                "destOverride": ["http", "tls"]
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom",
            "tag": "direct"
        },
        {
            "protocol": "blackhole",
            "tag": "block"
        }
    ],
    "policy": {
        "levels": {
            "0": {
                "handshake": 2,
                "connIdle": 120
            }
        }
    }
}
```

#### 客户端配置
```jsonc
{
    "log": {
        "loglevel": "warning"
    },
    "routing": {
        "domainStrategy": "IPIfNonMatch",
        "rules": [
            {
                "domain": ["geosite:cn", "geosite:private"],
                "outboundTag": "direct"
            },
            {
                "ip": ["geoip:cn", "geoip:private"],
                "outboundTag": "direct"
            }
        ]
    },
    "inbounds": [
        {
            "listen": "127.0.0.1",
            "port": 10808,
            "protocol": "socks",
            "settings": {
                "udp": true
            },
            "sniffing": {
                "enabled": true,
                "destOverride": ["http", "tls"]
            }
        },
        {
            "listen": "127.0.0.1",
            "port": 10809,
            "protocol": "http",
            "sniffing": {
                "enabled": true,
                "destOverride": ["http", "tls"]
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "vless",
            "settings": {
                "vnext": [
                    {
                        "address": "your-server.com",
                        "port": 443,
                        "users": [
                            {
                                "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
                                "encryption": "none",
                                "flow": "xtls-rprx-vision"
                            }
                        ]
                    }
                ]
            },
            "streamSettings": {
                "network": "tcp",
                "security": "tls",
                "tlsSettings": {
                    "serverName": "your-server.com",
                    "allowInsecure": false,
                    "fingerprint": "chrome"
                }
            },
            "tag": "proxy"
        },
        {
            "protocol": "freedom",
            "tag": "direct"
        }
    ]
}
```

### 2. VLESS-REALITY 配置

#### 服务器端配置
```jsonc
{
    "log": {
        "loglevel": "debug"
    },
    "inbounds": [
        {
            "port": 443,
            "protocol": "vless",
            "settings": {
                "clients": [
                    {
                        "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",  // xray uuid
                        "flow": "xtls-rprx-vision"
                    }
                ],
                "decryption": "none"
            },
            "streamSettings": {
                "network": "tcp",
                "security": "reality",
                "realitySettings": {
                    "dest": "www.microsoft.com:443",  // Target website with TLS 1.3 and HTTP/2
                    "serverNames": ["www.microsoft.com"],
                    "privateKey": "sK8FvJCpQ2YtN6MlXUz3RdE7W9gT4hB5nLx0pV",  // xray x25519
                    "shortIds": [
                        "",
                        "0123456789abcdef"
                    ]
                }
            },
            "sniffing": {
                "enabled": true,
                "destOverride": ["http", "tls", "quic"],
                "routeOnly": true
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom",
            "tag": "direct"
        }
    ]
}
```

#### 客户端配置
```jsonc
{
    "log": {
        "loglevel": "warning"
    },
    "inbounds": [
        {
            "listen": "127.0.0.1",
            "port": 10808,
            "protocol": "socks",
            "settings": {
                "udp": true
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "vless",
            "settings": {
                "vnext": [
                    {
                        "address": "203.0.113.10",
                        "port": 443,
                        "users": [
                            {
                                "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
                                "encryption": "none",
                                "flow": "xtls-rprx-vision"
                            }
                        ]
                    }
                ]
            },
            "streamSettings": {
                "network": "tcp",
                "security": "reality",
                "realitySettings": {
                    "show": false,
                    "fingerprint": "chrome",
                    "serverName": "www.microsoft.com",
                    "publicKey": "xK3FhN9pM2wL5vT7R8bE4cD6gH0jZ",  // Corresponding public key
                    "shortId": "0123456789abcdef",
                    "spiderX": "/"
                }
            },
            "tag": "proxy"
        }
    ]
}
```

### 3. VLESS-gRPC 配置

#### 服务器端配置
```jsonc
{
    "log": {
        "loglevel": "warning"
    },
    "routing": {
        "domainStrategy": "IPIfNonMatch",
        "rules": [
            {
                "port": "80",
                "network": "udp",
                "outboundTag": "block"
            },
            {
                "ip": ["geoip:private"],
                "outboundTag": "block"
            }
        ]
    },
    "inbounds": [
        {
            "listen": "0.0.0.0",
            "port": 80,
            "protocol": "vless",
            "settings": {
                "clients": [
                    {
                        "id": "drEwvgYhS15C",
                        "flow": ""
                    }
                ],
                "decryption": "none"
            },
            "streamSettings": {
                "network": "grpc",
                "security": "reality",
                "realitySettings": {
                    "show": false,
                    "dest": "www.yahoo.com:443",
                    "xver": 0,
                    "serverNames": ["www.yahoo.com", "news.yahoo.com"],
                    "privateKey": "kOsBHSgxhAfCeQIQyJvupiXTmQrMmsqi6y6Wc5OQZXc",
                    "shortIds": ["d49d578f280fd83a"]
                },
                "grpcSettings": {
                    "serviceName": ""
                }
            },
            "sniffing": {
                "enabled": true,
                "destOverride": ["http", "tls", "quic"]
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom",
            "tag": "direct"
        },
        {
            "protocol": "blackhole",
            "tag": "block"
        }
    ],
    "policy": {
        "levels": {
            "0": {
                "handshake": 2,
                "connIdle": 120
            }
        }
    }
}
```

### 4. VLESS 客户端链接示例

#### TCP
```
vless://90e4903e-66a4-45f7-abda-fd5d5ed7f797@example.com:443?security=tls&type=tcp#Vless-TCP
```

#### WebSocket
```
vless://90e4903e-66a4-45f7-abda-fd5d5ed7f797@example.com:443?security=tls&type=ws&path=/vlws#Vless-WS
```

#### gRPC
```
vless://90e4903e-66a4-45f7-abda-fd5d5ed7f797@example.com:443?security=tls&type=grpc&serviceName=vlgrpc#Vless-gRPC
```

#### HTTP/2
```
vless://90e4903e-66a4-45f7-abda-fd5d5ed7f797@example.com:443?sni=vlh2o.example.com&security=tls&type=http&path=/vlh2#Vless-H2
```

---

## VMess 协议配置

### 1. VMess 基础 TCP 配置

#### 服务器端配置
```json
{
    "log": {
        "loglevel": "warning"
    },
    "inbounds": [
        {
            "port": 1234,
            "protocol": "vmess",
            "settings": {
                "clients": [
                    {
                        "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
                        "alterId": 0
                    }
                ]
            },
            "streamSettings": {
                "network": "tcp"
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom"
        }
    ]
}
```

#### 客户端配置
```json
{
    "log": {
        "loglevel": "warning"
    },
    "routing": {
        "domainStrategy": "AsIs",
        "rules": [
            {
                "ip": ["geoip:private"],
                "outboundTag": "direct"
            }
        ]
    },
    "inbounds": [
        {
            "listen": "127.0.0.1",
            "port": 1080,
            "protocol": "socks",
            "settings": {
                "auth": "noauth",
                "udp": true,
                "ip": "127.0.0.1"
            }
        },
        {
            "listen": "127.0.0.1",
            "port": 1081,
            "protocol": "http"
        }
    ],
    "outbounds": [
        {
            "protocol": "vmess",
            "settings": {
                "vnext": [
                    {
                        "address": "server.example.com",
                        "port": 1234,
                        "users": [
                            {
                                "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef"
                            }
                        ]
                    }
                ]
            },
            "streamSettings": {
                "network": "tcp"
            },
            "tag": "proxy"
        },
        {
            "protocol": "freedom",
            "tag": "direct"
        }
    ]
}
```

### 2. VMess 客户端链接示例

#### TCP (Base64 编码)
```
vmess://ewogICAgImFkZCI6ICJleGFtcGxlLmNvbSIsCiAgICAiYWlkIjogIjAiLAogICAgImhvc3QiOiAiIiwKICAgICJpZCI6ICI5MGU0OTAzZS02NmE0LTQ1ZjctYWJkYS1mZDVkNWVkN2Y3OTciLAogICAgIm5ldCI6ICJ0Y3AiLAogICAgInBhdGgiOiAiL3ZtdGMiLAogICAgInBvcnQiOiAiNDQzIiwKICAgICJwcyI6ICJWTUVTUy1UQ1AiLAogICAgInNjeSI6ICJub25lIiwKICAgICJzbmkiOiAiIiwKICAgICJ0bHMiOiAidGxzIiwKICAgICx0eXBlIjogImh0dHAiLAogICAgInYiOiAiMiIKfQo=
```

#### WebSocket (Base64 编码)
```
vmess://ewogICAgImFkZCI6ICJleGFtcGxlLmNvbSIsCiAgICAiYWlkIjogIjAiLAogICAgImhvc3QiOiAiIiwKICAgICJpZCI6ICI5MGU0OTAzZS02NmE0LTQ1ZjctYWJkYS1mZDVkNWVkN2Y3OTciLAogICAgIm5ldCI6ICJ3cyIsCiAgICAicGF0aCI6ICIvdm13cyIsCiAgICAicG9ydCI6ICI0NDM=LAogICAgInBzIjogIlZNRVNTLVdTIiwKICAgICJzY3kiOiAibm9uZSIsCiAgICAic25pIjogIiIsCiAgICAidGxzIjogInRscyIsCiAgICAidHlwZSI6ICIiLAogICAgInYiOiAiMiIKfQo=
```

#### gRPC (Base64 编码)
```
vmess://ewogICAgImFkZCI6ICJleGFtcGxlLmNvbSIsCiAgICAiYWlkIjogIjAiLAogICAgImhvc3QiOiAiIiwKICAgICJpZCI6ICI5MGU0OTAzZS02NmE0LTQ1ZjctYWJkYS1mZDVkNWVkN2Y3OTciLAogICAgIm5ldCI6ICJncnBjIiwKICAgICJwYXRoIjogInZtZ3JwYyIsCiAgICAicG9ydCI6ICI0NDMiLAogICAgInBzIjogIlZNRVNTLWdSUEMiLAogICAgInNjeSI6ICJub25lIiwKICAgICJzbmkiOiAiIiwKICAgICJ0bHMiOiAidGxzIiwKICAgICJ0eXBlIjogImh0dHAiLAogICAgInYiOiAiMiIKfQo=
```

#### HTTP/2 (Base64 编码)
```
vmess://ewogICAgImFkZCI6ICJleGFtcGxlLmNvbSIsCiAgICAiYWlkIjogIjAiLAogICAgImhvc3QiOiAiIiwKICAgICJpZCI6ICI5MGU0OTAzZS02NmE0LTQ1ZjctYWJkYS1mZDVkNWVkN2Y3OTciLAogICAgIm5ldCI6ICJodHRwIiwKICAgICJwYXRoIjogIi92bWgyIiwKICAgICJwb3J0IjogIjQ0MyIsCiAgICAicHMiOiAiVk1FU1MtSDIiLAogICAgInNjeSI6ICJub25lIiwKICAgICJzbmkiOiAidm1oMm8uZXhhbXBsZS5jb20iLAogICAgInRscyI6ICJ0bHMiLAogICAgInR5cGUiOiAiaHR0cCI=LAogICAgInYiOiAiMiIKfQo=
```

---

## Trojan 协议配置

### 1. Trojan 客户端链接示例

#### TCP
```
trojan://desdemona99@example.com:443?security=tls&type=tcp#Trojan-TCP
```

#### WebSocket
```
trojan://desdemona99@example.com:443?security=tls&type=ws&path=/trojanws#Trojna-WS
```

#### gRPC
```
trojan://desdemona99@example.com:443?security=tls&type=grpc&serviceName=trgrpc#Trojan-gRPC
```

#### HTTP/2
```
trojan://desdemona99@example.com:443?sni=trh2o.example.com&security=tls&type=http&path=/trh2#Trojan-H2
```

---

## Shadowsocks 协议配置

### 1. Shadowsocks 2022 单用户配置

#### 服务器端配置
```json
{
   "inbounds": [
     {
       "port": 1234,
       "protocol": "shadowsocks",
       "settings": {
         "method": "2022-blake3-aes-128-gcm",
         "password": "{{ psk }}",
         "network": "tcp,udp"
       }
     }
   ],
   "outbounds": [
     {
       "protocol": "freedom"
     }
   ]
}
```

#### 客户端配置
```json
{
   "inbounds": [
     {
       "port": 10801,
       "protocol": "socks",
       "settings": {
         "udp": true
       }
     },
     {
       "port": 10802,
       "protocol": "http"
     }
   ],
   "outbounds": [
     {
       "protocol": "shadowsocks",
       "settings": {
         "servers": [
           {
             "address": "{{ host }}",
             "port": 1234,
             "method": "2022-blake3-aes-128-gcm",
             "password": "{{ psk }}"
           }
         ]
       }
     }
   ]
}
```

### 2. Shadowsocks 2022 多用户配置

#### 服务器端配置
```json
{
   "inbounds": [
     {
       "port": 1234,
       "protocol": "shadowsocks",
       "settings": {
         "method": "2022-blake3-aes-128-gcm",
         "password": "{{ server psk }}",
         "clients": [
           {
             "password": "{{ user psk }}",
             "email": "my user"
           }
         ],
         "network": "tcp,udp"
       }
     }
   ],
   "outbounds": [
     {
       "protocol": "freedom"
     }
   ]
}
```

#### 客户端配置
```json
{
   "inbounds": [
     {
       "port": 10801,
       "protocol": "socks",
       "settings": {
         "udp": true
       }
     },
     {
       "port": 10802,
       "protocol": "http"
     }
   ],
   "outbounds": [
     {
       "protocol": "shadowsocks",
       "settings": {
         "servers": [
           {
             "address": "{{ host }}",
             "port": 1234,
             "method": "2022-blake3-aes-128-gcm",
             "password": "{{ server psk }}:{{ user psk }}"
           }
         ]
       }
     }
   ]
}
```

### 3. Shadowsocks 2022 UDP over TCP 配置

#### 客户端配置
```json
{
   "inbounds": [
     {
       "port": 10801,
       "protocol": "socks",
       "settings": {
         "udp": true
       }
     },
     {
       "port": 10802,
       "protocol": "http"
     }
   ],
   "outbounds": [
     {
       "protocol": "shadowsocks",
       "settings": {
         "servers": [
           {
             "address": "{{ host }}",
             "port": 1234,
             "method": "2022-blake3-aes-128-gcm",
             "password": "{{ psk }}",
             "uot": true
           }
         ]
       }
     }
   ]
}
```

### 4. Shadowsocks AEAD 配置

#### 服务器端配置
```json
{
     "inbounds": [
         {
             "port": 12345,
             "protocol": "shadowsocks",
             "settings": {
                 "clients": [
                     {
                         "password": "example_user_1",
                         "method": "aes-128-gcm"
                     },
                     {
                         "password": "example_user_2",
                         "method": "aes-256-gcm"
                     },
                     {
                         "password": "example_user_3",
                         "method": "chacha20-poly1305"
                     }
                 ],
                 "network": "tcp,udp"
             }
         }
     ],
     "outbounds": [
         {
             "protocol": "freedom"
         }
     ]
}
```

#### 客户端配置
```json
{
     "inbounds": [
         {
             "port": 10801,
             "protocol": "socks",
             "settings": {
                 "udp": true
             }
         },
         {
             "port": 10802,
             "protocol": "http"
         }
     ],
     "outbounds": [
         {
             "protocol": "shadowsocks",
             "settings": {
                 "servers": [
                     {
                         "address": "",
                         "port": 12345,
                         "password": "example_user_1",
                         "method": "aes-128-gcm"
                     }
                 ]
             }
         }
     ]
}
```

---

## 关键配置参数说明

### Reality 配置参数
- **serverNames**: 服务器域名列表，用于验证 SNI
- **shortIds**: 短 ID 列表，用于客户端识别
- **publicKey**: 公钥，客户端用于验证
- **privateKey**: 私钥，服务器端配置
- **dest**: 目标网站，Reality 伪装的网站

### Flow 配置
- **xtls-rprx-vision**: 推荐的 XTLS 流控模式
- **xtls-rprx-vision-udp443**: 支持 UDP over TCP 的 XTLS 模式

### 安全层配置
- **tls**: 传统 TLS 加密
- **reality**: 无需证书的伪装协议
- **fingerprint**: 指纹模拟，通常使用 "chrome"

### 传输层配置
- **tcp**: 基础 TCP 传输
- **ws**: WebSocket 传输
- **grpc**: gRPC 传输
- **http**: HTTP/2 传输

### 路由配置
- **geosite:cn**: 中国大陆网站
- **geoip:cn**: 中国大陆 IP
- **geoip:private**: 私有 IP 段

---

## Nginx 配置示例

### VLESS-gRPC Nginx 配置
```nginx
server {
listen 443 ssl http2 so_keepalive=on;
listen [::]:443 ssl http2 so_keepalive=on;
server_name example.com;

index index.html;
root /var/www/html;

ssl_certificate /path/to/example.cer;
ssl_certificate_key /path/to/example.key;
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE -RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384;

client_header_timeout 52w;
         keepalive_timeout 52w;
# Fill in /your ServiceName after location
location /your ServiceName {
if ($content_type !~ "application/grpc") {
return 404;
}
client_max_body_size 0;
client_body_buffer_size 512k;
grpc_set_header X-Real-IP $remote_addr;
client_body_timeout 52w;
grpc_read_timeout 52w;
grpc_pass unix:/dev/shm/Xray-VLESS-gRPC.socket;
}
}
```

---

*本文档配置示例来源于 [XTLS/Xray-examples](https://github.com/XTLS/Xray-examples) 仓库，仅供学习和参考使用。*