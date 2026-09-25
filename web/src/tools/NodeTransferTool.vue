<script setup lang="ts">
import { ref, computed, inject } from "vue";
import QrcodeVue from "qrcode.vue";

const toast =
  inject<
    (message: string, type?: "info" | "error" | "success" | "warning") => void
  >("toast");
const ElMessage = {
  success: (message: string) => toast?.(message, "success"),
  error: (message: string) => toast?.(message, "error"),
};

const SAMPLE_VLESS_JSON = `{
  "type": "vless",
  "tag": "rjp-reality",
  "server": "rjp.wowoha.top",
  "server_port": 8443,
  "uuid": "45d357f4-0540-41ed-8e0d-10806b70b386",
  "flow": "xtls-rprx-vision",
  "tls": {
    "enabled": true,
    "server_name": "www.amazon.com",
    "utls": {
      "enabled": true,
      "fingerprint": "chrome"
    },
    "reality": {
      "enabled": true,
      "public_key": "PjePONzwgc9HCWQf209unDj_gVwaqX4c-a2hgi4t2R0",
      "short_id": "0123456789abcdef"
    }
  },
  "domain_resolver": "ali"
}`;

const SAMPLE_TUIC_JSON = `{
  "type": "tuic",
  "tag": "rjp-tuic",
  "server": "rjp.wowoha.top",
  "server_port": 8443,
  "uuid": "45d357f4-0540-41ed-8e0d-10806b70b386",
  "password": "my-secret-password",
  "congestion_control": "bbr",
  "udp_relay_mode": "native",
  "alpn": [
    "h3"
  ],
  "tls": {
    "enabled": true,
    "server_name": "www.amazon.com",
    "insecure": false
  }
}`;

const direction = ref<"json2link" | "link2json">("json2link");
const inputText = ref("");

// 优先根据输入内容判断转换方向，direction 仅作为空内容或无法识别时的默认方向。
const conversionDirection = computed<"json2link" | "link2json">(() => {
  const value = inputText.value.trim();
  if (!value) return direction.value;
  if (/^[a-z][a-z\d+.-]*:\/\//i.test(value)) return "link2json";
  if (value.startsWith("{") || value.startsWith("[")) return "json2link";
  try {
    JSON.parse(value);
    return "json2link";
  } catch {
    return direction.value;
  }
});

const SAMPLE_VLESS_LINK =
  "vless://45d357f4-0540-41ed-8e0d-10806b70b386@rjp.wowoha.top:8443?flow=xtls-rprx-vision&security=reality&sni=www.amazon.com&fp=chrome&pbk=PjePONzwgc9HCWQf209unDj_gVwaqX4c-a2hgi4t2R0&sid=0123456789abcdef#rjp-reality";

const SAMPLE_TUIC_LINK =
  "tuic://45d357f4-0540-41ed-8e0d-10806b70b386:my-secret-password@rjp.wowoha.top:8443?congestion_control=bbr&udp_relay_mode=native&alpn=h3&sni=www.amazon.com#rjp-tuic";

const parseVlessUrl = (urlStr: string) => {
  const url = new URL(urlStr);
  if (url.protocol !== "vless:") {
    throw new Error("协议不正确，必须为 vless://");
  }

  const uuid = url.username;
  const server = url.hostname;
  const portStr = url.port;
  const port = portStr ? parseInt(portStr) : 443;
  const tag = url.hash ? decodeURIComponent(url.hash.substring(1)) : "vless";

  if (!uuid) {
    throw new Error("链接中缺少 UUID");
  }
  if (!server) {
    throw new Error("链接中缺少服务器地址 (server)");
  }
  if (!portStr) {
    throw new Error("链接中缺少服务器端口 (port)");
  }

  const searchParams = url.searchParams;
  const flow = searchParams.get("flow");
  const security = searchParams.get("security");
  const sni = searchParams.get("sni");
  const fp = searchParams.get("fp");
  const pbk = searchParams.get("pbk");
  const sid = searchParams.get("sid");

  const config: any = {
    type: "vless",
    tag: tag,
    server: server,
    server_port: port,
    uuid: uuid,
  };

  if (flow) {
    config.flow = flow;
  }

  // TLS & Reality
  if (security === "reality" || security === "tls" || sni || fp || pbk || sid) {
    config.tls = {
      enabled: true,
    };
    if (sni) {
      config.tls.server_name = sni;
    }
    if (fp) {
      config.tls.utls = {
        enabled: true,
        fingerprint: fp,
      };
    }
    if (security === "reality" || pbk || sid) {
      config.tls.reality = {
        enabled: true,
        public_key: pbk || "",
        short_id: sid || "",
      };
    }
  }

  // Transport
  const transportType = searchParams.get("type");
  const validTransports = ["ws", "http", "grpc", "quic", "httpupgrade"];
  if (transportType && validTransports.includes(transportType)) {
    config.transport = {
      type: transportType,
    };
    if (
      transportType === "ws" ||
      transportType === "http" ||
      transportType === "httpupgrade"
    ) {
      const path = searchParams.get("path");
      if (path) {
        config.transport.path = decodeURIComponent(path);
      }
      const host = searchParams.get("host");
      if (host) {
        config.transport.host = [decodeURIComponent(host)];
      }
    } else if (transportType === "grpc") {
      const serviceName = searchParams.get("serviceName");
      if (serviceName) {
        config.transport.service_name = decodeURIComponent(serviceName);
      }
    }
  }

  return config;
};

const parseTuicUrl = (urlStr: string) => {
  const url = new URL(urlStr);
  if (url.protocol !== "tuic:") {
    throw new Error("协议不正确，必须为 tuic://");
  }

  const uuid = url.username;
  const password = decodeURIComponent(url.password || "");
  const server = url.hostname;
  const portStr = url.port;
  const port = portStr ? parseInt(portStr) : 443;
  const tag = url.hash ? decodeURIComponent(url.hash.substring(1)) : "tuic";

  if (!uuid) {
    throw new Error("链接中缺少 UUID");
  }
  if (!password) {
    throw new Error("链接中缺少密码 (password)");
  }
  if (!server) {
    throw new Error("链接中缺少服务器地址 (server)");
  }
  if (!portStr) {
    throw new Error("链接中缺少服务器端口 (port)");
  }

  const searchParams = url.searchParams;
  const congestionControl = searchParams.get("congestion_control");
  const udpRelayMode = searchParams.get("udp_relay_mode");
  const alpn = searchParams.get("alpn");
  const sni = searchParams.get("sni");
  const allowInsecure = searchParams.get("allow_insecure");

  const config: any = {
    type: "tuic",
    tag: tag,
    server: server,
    server_port: port,
    uuid: uuid,
    password: password,
  };

  if (congestionControl) {
    config.congestion_control = congestionControl;
  }
  if (udpRelayMode) {
    config.udp_relay_mode = udpRelayMode;
  }

  // ALPN & TLS
  const hasTlsParams = alpn || sni || allowInsecure === "1";
  if (hasTlsParams) {
    config.tls = {
      enabled: true,
    };
    if (sni) {
      config.tls.server_name = sni;
    }
    if (allowInsecure === "1") {
      config.tls.insecure = true;
    }
  }

  if (alpn) {
    config.alpn = alpn
      .split(",")
      .map((item) => decodeURIComponent(item.trim()));
  }

  return config;
};

const parseLinkToConfig = (link: string) => {
  const cleanLink = link.trim();
  if (!cleanLink) {
    throw new Error("输入内容为空");
  }
  if (cleanLink.startsWith("vless://")) {
    return parseVlessUrl(cleanLink);
  } else if (cleanLink.startsWith("tuic://")) {
    return parseTuicUrl(cleanLink);
  }
  throw new Error("未知的分享链接格式，必须以 vless:// 或 tuic:// 开头");
};

const handlePasteSample = (type: "vless" | "tuic") => {
  if (conversionDirection.value === "json2link") {
    inputText.value = type === "vless" ? SAMPLE_VLESS_JSON : SAMPLE_TUIC_JSON;
  } else {
    inputText.value = type === "vless" ? SAMPLE_VLESS_LINK : SAMPLE_TUIC_LINK;
  }
};

const handleClear = () => {
  inputText.value = "";
};

const handleDirectionChange = (newDir: "json2link" | "link2json") => {
  direction.value = newDir;
  inputText.value = "";
};

const handleFormat = () => {
  try {
    if (!inputText.value.trim()) return;
    const parsed = JSON.parse(inputText.value);
    inputText.value = JSON.stringify(parsed, null, 2);
  } catch (e: any) {
    ElMessage.error(`格式化失败: ${e.message}`);
  }
};

const copyToClipboard = (text: string) => {
  if (!text) return;
  navigator.clipboard.writeText(text).then(
    () => {
      ElMessage.success("已复制到剪贴板");
    },
    () => {
      ElMessage.error("复制失败，请手动选择复制");
    },
  );
};

const parsedConfig = computed(() => {
  if (!inputText.value.trim()) return null;
  try {
    if (conversionDirection.value === "json2link") {
      const raw = JSON.parse(inputText.value);

      let config = raw;
      if (raw && typeof raw === "object") {
        if (Array.isArray(raw)) {
          config =
            raw.find(
              (item: any) =>
                item && (item.type === "vless" || item.type === "tuic"),
            ) || raw[0];
        } else if (Array.isArray(raw.outbounds)) {
          config =
            raw.outbounds.find(
              (item: any) =>
                item && (item.type === "vless" || item.type === "tuic"),
            ) || raw.outbounds[0];
        }
      }

      return config;
    } else {
      return parseLinkToConfig(inputText.value);
    }
  } catch (e) {
    return null;
  }
});

const parseError = computed(() => {
  if (!inputText.value.trim()) return "";
  try {
    if (conversionDirection.value === "json2link") {
      const raw = JSON.parse(inputText.value);
      let config = raw;
      if (raw && typeof raw === "object") {
        if (Array.isArray(raw)) {
          config = raw.find(
            (item: any) =>
              item && (item.type === "vless" || item.type === "tuic"),
          );
          if (!config) return "数组中未找到 type 为 vless 或 tuic 的出站配置";
        } else if (Array.isArray(raw.outbounds)) {
          config = raw.outbounds.find(
            (item: any) =>
              item && (item.type === "vless" || item.type === "tuic"),
          );
          if (!config)
            return "配置文件的 outbounds 中未找到 type 为 vless 或 tuic 的出站配置";
        }
      }
      if (!config || typeof config !== "object") {
        return "配置必须是一个 JSON 对象";
      }
      const supportedTypes = ["vless", "tuic"];
      if (!supportedTypes.includes(config.type)) {
        return `配置类型必须为 vless 或 tuic，当前为: ${config.type || "未指定"}`;
      }
      if (!config.server) {
        return "缺少 server (服务器地址) 字段";
      }
      if (!config.server_port) {
        return "缺少 server_port (服务器端口) 字段";
      }
      if (!config.uuid) {
        return "缺少 uuid 字段";
      }
      if (config.type === "tuic" && !config.password) {
        return "缺少 password 字段 (TUIC 协议必填)";
      }
      return "";
    } else {
      parseLinkToConfig(inputText.value);
      return "";
    }
  } catch (e: any) {
    if (conversionDirection.value === "json2link") {
      return `JSON 解析错误: ${e.message}`;
    } else {
      return `链接解析错误: ${e.message}`;
    }
  }
});

const shareLinkJson = computed(() => {
  const config = parsedConfig.value;
  const err = parseError.value;
  if (!config || err) return "";
  return JSON.stringify(config, null, 2);
});

const shareLink = computed(() => {
  const config = parsedConfig.value;
  const err = parseError.value;
  if (!config || err) return "";

  if (config.type === "vless") {
    const uuid = config.uuid || "";
    const server = config.server || "";
    const port = config.server_port || "";
    const tag = encodeURIComponent(config.tag || "vless");

    const params: string[] = [];

    // Flow
    if (config.flow) {
      params.push(`flow=${config.flow}`);
    }

    // Security & TLS & Reality
    let security = "none";
    if (config.tls?.enabled) {
      if (config.tls.reality?.enabled) {
        security = "reality";
      } else {
        security = "tls";
      }
    }
    params.push(`security=${security}`);

    // SNI
    if (config.tls?.server_name) {
      params.push(`sni=${config.tls.server_name}`);
    }

    // uTLS Fingerprint
    if (config.tls?.utls?.enabled && config.tls.utls.fingerprint) {
      params.push(`fp=${config.tls.utls.fingerprint}`);
    }

    // Reality parameters
    if (security === "reality" && config.tls?.reality) {
      const pbk = config.tls.reality.public_key;
      if (pbk) {
        params.push(`pbk=${pbk}`);
      }
      const sid = config.tls.reality.short_id;
      if (sid) {
        params.push(`sid=${sid}`);
      }
    }

    // Transport settings
    const validTransports = ["ws", "http", "grpc", "quic", "httpupgrade"];
    if (
      config.transport?.type &&
      validTransports.includes(config.transport.type)
    ) {
      const transportType = config.transport.type;
      params.push(`type=${transportType}`);

      if (
        transportType === "ws" ||
        transportType === "http" ||
        transportType === "httpupgrade"
      ) {
        if (config.transport.path) {
          params.push(`path=${encodeURIComponent(config.transport.path)}`);
        }

        // Extract host
        let host = "";
        if (typeof config.transport.host === "string") {
          host = config.transport.host;
        } else if (
          Array.isArray(config.transport.host) &&
          config.transport.host.length > 0
        ) {
          host = config.transport.host[0];
        } else if (config.transport.headers) {
          const hostKey = Object.keys(config.transport.headers).find(
            (k) => k.toLowerCase() === "host",
          );
          if (hostKey) {
            host = config.transport.headers[hostKey];
          }
        }

        if (host) {
          params.push(`host=${encodeURIComponent(host)}`);
        }
      } else if (transportType === "grpc") {
        if (config.transport.service_name) {
          params.push(
            `serviceName=${encodeURIComponent(config.transport.service_name)}`,
          );
        }
      }
    }

    const queryString = params.length > 0 ? `?${params.join("&")}` : "";
    return `vless://${uuid}@${server}:${port}${queryString}#${tag}`;
  } else if (config.type === "tuic") {
    const uuid = config.uuid || "";
    const password = config.password || "";
    const server = config.server || "";
    const port = config.server_port || "";
    const tag = encodeURIComponent(config.tag || "tuic");

    const params: string[] = [];

    if (config.congestion_control) {
      params.push(`congestion_control=${config.congestion_control}`);
    }
    if (config.udp_relay_mode) {
      params.push(`udp_relay_mode=${config.udp_relay_mode}`);
    }

    // ALPN
    if (Array.isArray(config.alpn) && config.alpn.length > 0) {
      params.push(`alpn=${encodeURIComponent(config.alpn.join(","))}`);
    } else if (
      config.tls?.alpn &&
      Array.isArray(config.tls.alpn) &&
      config.tls.alpn.length > 0
    ) {
      params.push(`alpn=${encodeURIComponent(config.tls.alpn.join(","))}`);
    }

    // SNI
    if (config.tls?.server_name) {
      params.push(`sni=${config.tls.server_name}`);
    }

    // Allow Insecure
    if (config.tls?.insecure) {
      params.push(`allow_insecure=1`);
    }

    const queryString = params.length > 0 ? `?${params.join("&")}` : "";
    return `tuic://${uuid}:${password}@${server}:${port}${queryString}#${tag}`;
  }

  return "";
});

const configDetails = computed(() => {
  const config = parsedConfig.value;
  if (!config || parseError.value) return [];

  const details = [
    { label: "出站协议 (type)", value: config.type, highlight: true },
    { label: "别名标识 (tag)", value: config.tag || "-" },
    { label: "服务器地址 (server)", value: config.server },
    { label: "端口 (server_port)", value: config.server_port },
    { label: "UUID", value: config.uuid, copyable: true },
  ];

  if (config.type === "vless") {
    details.push({ label: "流控算法 (flow)", value: config.flow || "none" });

    // TLS Info
    const tlsEnabled = !!config.tls?.enabled;
    details.push({
      label: "TLS 状态",
      value: tlsEnabled ? "已启用 (Enabled)" : "未启用 (Disabled)",
    });

    if (tlsEnabled) {
      details.push({
        label: "TLS SNI (server_name)",
        value: config.tls.server_name || "-",
      });

      const utlsEnabled = !!config.tls.utls?.enabled;
      details.push({
        label: "uTLS 混淆指纹",
        value: utlsEnabled
          ? `已启用 (${config.tls.utls.fingerprint || "none"})`
          : "未启用",
      });

      const realityEnabled = !!config.tls.reality?.enabled;
      details.push({
        label: "Reality 混淆",
        value: realityEnabled ? "已启用 (REALITY)" : "未启用",
      });

      if (realityEnabled && config.tls.reality) {
        details.push({
          label: "Reality 公钥 (pbk)",
          value: config.tls.reality.public_key || "-",
          copyable: true,
        });
        details.push({
          label: "Reality 短 ID (sid)",
          value: config.tls.reality.short_id || "-",
        });
      }
    }

    // Transport Info
    if (config.transport?.type) {
      details.push({
        label: "传输层协议 (type)",
        value: config.transport.type,
      });
      if (config.transport.path) {
        details.push({
          label: "传输路径 (path)",
          value: config.transport.path,
        });
      }

      let host = "";
      if (typeof config.transport.host === "string") {
        host = config.transport.host;
      } else if (
        Array.isArray(config.transport.host) &&
        config.transport.host.length > 0
      ) {
        host = config.transport.host[0];
      } else if (config.transport.headers) {
        const hostKey = Object.keys(config.transport.headers).find(
          (k) => k.toLowerCase() === "host",
        );
        if (hostKey) host = config.transport.headers[hostKey];
      }
      if (host) {
        details.push({ label: "传输主机 (host)", value: host });
      }

      if (config.transport.service_name) {
        details.push({
          label: "gRPC 服务名 (service_name)",
          value: config.transport.service_name,
        });
      }
    }
  } else if (config.type === "tuic") {
    details.push(
      {
        label: "密码 (password)",
        value: config.password || "-",
        copyable: true,
      },
      {
        label: "拥塞控制 (congestion_control)",
        value: config.congestion_control || "bbr",
      },
      {
        label: "UDP 转发模式 (udp_relay_mode)",
        value: config.udp_relay_mode || "native",
      },
    );

    let alpn = "-";
    if (Array.isArray(config.alpn)) {
      alpn = config.alpn.join(", ");
    } else if (config.tls?.alpn && Array.isArray(config.tls.alpn)) {
      alpn = config.tls.alpn.join(", ");
    }
    details.push({ label: "ALPN", value: alpn });
    details.push({
      label: "TLS SNI (server_name)",
      value: config.tls?.server_name || "-",
    });
    details.push({
      label: "允许不安全 TLS (insecure)",
      value: config.tls?.insecure ? "是 (true)" : "否 (false)",
    });
  }

  return details;
});
</script>

<template>
  <section class="transfer-tool">
    <header class="tool-intro">
      <div>
        <h2>节点配置转换</h2>
        <p>在 Sing-box JSON 与 VLESS / TUIC 分享链接之间快速转换。</p>
      </div>
      <div class="flex flex-1 justify-center">
        <div class="direction-switch">
          <button
            :class="{ active: conversionDirection === 'json2link' }"
            @click="handleDirectionChange('json2link')"
          >
            JSON <b>→</b> 分享链接
          </button>
          <button
            :class="{ active: conversionDirection === 'link2json' }"
            @click="handleDirectionChange('link2json')"
          >
            分享链接 <b>→</b> JSON
          </button>
        </div>
      </div>
      <div class="flex-1"></div>
    </header>

    <div
      v-if="parseError"
      class="parse-error flex-2 w-fit"
      role="alert"
      aria-live="polite"
    >
      <span class="parse-error-icon">!</span>
      <div>
        <strong>解析配置有误</strong><span>{{ parseError }}</span>
      </div>
    </div>

    <div class="transfer-grid">
      <section class="transfer-card input-card">
        <div class="card-heading">
          <div>
            <h3>
              {{
                conversionDirection === "json2link"
                  ? "Sing-box 配置 JSON"
                  : "VLESS / TUIC 分享链接"
              }}
            </h3>
          </div>
          <div class="card-actions">
            <button @click="handlePasteSample('vless')">
              {{
                conversionDirection === "json2link"
                  ? "VLESS 示例"
                  : "VLESS 链接"
              }}</button
            ><button @click="handlePasteSample('tuic')">
              {{
                conversionDirection === "json2link" ? "TUIC 示例" : "TUIC 链接"
              }}</button
            ><button
              v-if="conversionDirection === 'json2link'"
              :disabled="!inputText"
              @click="handleFormat"
            >
              格式化</button
            ><button :disabled="!inputText" @click="handleClear">清空</button>
          </div>
        </div>

        <div class="editor-wrap">
          <textarea
            v-model="inputText"
            :placeholder="
              conversionDirection === 'json2link'
                ? '粘贴 sing-box 的 VLESS 或 TUIC 出站配置 JSON…'
                : '粘贴 vless:// 或 tuic:// 分享链接…'
            "
          ></textarea>
        </div>
        <footer>
          <span>{{ inputText.length }} 个字符</span
          ><span>{{
            conversionDirection === "json2link"
              ? "支持 sing-box 1.18+"
              : "支持 vless:// / tuic://"
          }}</span>
        </footer>
      </section>

      <section class="transfer-card output-card">
        <div class="card-heading">
          <div>
            <h3>
              {{
                conversionDirection === "json2link"
                  ? (parsedConfig?.type?.toUpperCase() || "节点") + " 标准链接"
                  : "Sing-box 配置 JSON"
              }}
            </h3>
          </div>
          <span v-if="parsedConfig && !parseError" class="valid-badge"
            >已识别</span
          >
        </div>
        <div v-if="parsedConfig && !parseError" class="output-content">
          <div class="result-box">
            <textarea
              readonly
              :value="
                conversionDirection === 'json2link' ? shareLink : shareLinkJson
              "
            ></textarea
            ><button
              @click="
                copyToClipboard(
                  conversionDirection === 'json2link'
                    ? shareLink
                    : shareLinkJson,
                )
              "
            >
              复制
            </button>
          </div>
          <div class="output-meta">
            <div v-if="shareLink" class="qr-box">
              <qrcode-vue
                :value="shareLink"
                :size="126"
                level="L"
                render-as="svg"
              /><small>扫码导入</small>
            </div>
            <div class="summary text-2xl">
              <dl class="w-full">
                <div class="grid grid-cols-3 gap-4 w-full">
                  <div>
                    <dt class="text-">节点名称</dt>
                    <dd>{{ parsedConfig?.tag || "未命名" }}</dd>
                  </div>
                  <div>
                    <dt>协议</dt>
                    <dd class="accent">
                      {{ parsedConfig?.type?.toUpperCase() }}
                    </dd>
                  </div>
                             <div>
                  <dt>服务器</dt>
                  <dd>
                    {{ parsedConfig?.server }}:{{ parsedConfig?.server_port }}
                  </dd>
                </div>
                </div>
     
              </dl>
            </div>
          </div>
        </div>
        <div v-else class="empty-output">
          <span>◎</span>
          <p>输入有效配置后，结果会显示在这里</p>
        </div>
      </section>
    </div>

    <section
      v-if="parsedConfig && !parseError"
      class="transfer-card details-card"
    >
      <div class="card-heading">
        <div>
          <span class="card-kicker">DETAILS</span>
          <h3>配置参数</h3>
        </div>
      </div>
      <div class="details-list">
        <div v-for="(detail, i) in configDetails" :key="i">
          <span>{{ detail.label }}</span
          ><strong :class="{ accent: detail.highlight }">{{
            detail.value
          }}</strong
          ><button
            v-if="detail.copyable && detail.value !== '-'"
            @click="copyToClipboard(detail.value)"
          >
            复制
          </button>
        </div>
      </div>
    </section>
  </section>
</template>

<style scoped>
.transfer-tool {
  display: grid;
  gap: 18px;
  max-width: 1180px;
  margin: 0 auto;
  color: var(--text-main);
}

.tool-intro {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.eyebrow,
.card-kicker {
  color: var(--primary);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.13em;
}

.tool-intro h2 {
  margin: 5px 0;
  font-size: 1.2rem;
}

.tool-intro p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.protocols {
  display: flex;
  gap: 6px;
}

.protocols span,
.valid-badge {
  padding: 5px 8px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--primary) 12%, transparent);
  color: var(--primary);
  font-size: 0.62rem;
  font-weight: 800;
}

.direction-switch {
  display: flex;
  gap: 4px;
  width: fit-content;
  padding: 4px;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  background: var(--bg-soft);
}

.direction-switch button,
.card-actions button {
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
  font-size: 0.68rem;
  font-weight: 700;
}

.direction-switch button {
  padding: 8px 13px;
}

.direction-switch button.active {
  background: var(--bg-elevated);
  color: var(--primary);
  box-shadow: 0 2px 8px rgba(2, 8, 23, 0.14);
}

.direction-switch b {
  margin: 0 4px;
  color: var(--text-dim);
}

.transfer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.transfer-card {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--border-glass);
  border-radius: 14px;
  background: var(--bg-panel);
}

.input-card,
.output-card {
  min-height: 460px;
}

.card-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 15px;
}

.card-heading h3 {
  margin: 4px 0 0;
  font-size: 0.9rem;
}

.card-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
}

.card-actions button {
  padding: 6px 8px;
  border-color: var(--border-glass);
}

.card-actions button:hover:not(:disabled) {
  border-color: var(--primary);
  color: var(--primary);
}

.card-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.editor-wrap {
  position: relative;
}

.editor-wrap textarea,
.result-box textarea {
  display: block;
  width: 100%;
  height: fit-content;
  box-sizing: border-box;
  resize: none;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  padding: 12px;
  outline: none;
  background: var(--bg-input);
  color: var(--text-main);
  font: 0.72rem/1.7 var(--font-mono);
}

.editor-wrap textarea {
  height: 340px;
}

.editor-wrap textarea:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent);
}

.editor-wrap textarea::placeholder {
  color: var(--text-dim);
}

.parse-error {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin: 0 0 10px;
  padding: 9px 11px;
  border: 1px solid color-mix(in srgb, var(--accent-rose) 35%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--accent-rose) 10%, var(--bg-panel));
  color: var(--accent-rose);
  font-size: 0.68rem;
}

.parse-error-icon {
  display: grid;
  flex: 0 0 17px;
  place-items: center;
  width: 17px;
  height: 17px;
  border-radius: 50%;
  background: var(--accent-rose);
  color: white;
  font-size: 0.65rem;
  font-weight: 800;
}

.parse-error div {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.parse-error span:last-child {
  overflow-wrap: anywhere;
  line-height: 1.4;
}

.transfer-card footer {
  display: flex;
  justify-content: space-between;
  margin-top: 10px;
  color: var(--text-dim);
  font-size: 0.64rem;
}

.output-content {
  display: grid;
  gap: 14px;
}

.result-box {
  position: relative;
}

.result-box textarea {
  height: 340px;
  min-height: 220px;
  padding-right: 70px;
}

.result-box button {
  position: absolute;
  top: 8px;
  right: 8px;
  border: 0;
  border-radius: 6px;
  padding: 6px 9px;
  background: var(--primary);
  color: white;
  cursor: pointer;
  font: inherit;
  font-size: 0.65rem;
  font-weight: 700;
}

.output-meta {
  display: grid;
  grid-template-columns: 150px minmax(0, 1fr);
  gap: 12px;
}

.qr-box,
.summary {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 7px;
  padding: 12px;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  background: var(--bg-input);
}

.qr-box svg {
  max-width: 100%;
  padding: 7px;
  background: white;
}

.qr-box small {
  color: var(--text-dim);
  font-size: 0.62rem;
}

.summary {
  align-content: start;
  justify-items: stretch;
}

.summary dl {
  display: grid;
  gap: 9px;
  margin: 11px 0;
}


.summary dt {
  color: var(--text-dim);
  font-size: 0.62rem;
}

.summary dd {
  overflow: hidden;
  margin: 0;
  color: var(--text-main);
  font: 0.7rem var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.accent {
  color: var(--primary) !important;
}

.summary button {
  border: 1px solid color-mix(in srgb, var(--primary) 28%, var(--border-glass));
  border-radius: 7px;
  padding: 7px;
  background: color-mix(in srgb, var(--primary) 9%, transparent);
  color: var(--primary);
  cursor: pointer;
  font: inherit;
  font-size: 0.65rem;
  font-weight: 700;
}

.empty-output {
  display: grid;
  min-height: 320px;
  place-items: center;
  align-content: center;
  gap: 10px;
  border: 1px dashed var(--border-glass);
  border-radius: 9px;
  color: var(--text-dim);
}

.empty-output span {
  font-size: 2rem;
  color: var(--primary);
}

.empty-output p {
  margin: 0;
  font-size: 0.7rem;
}

.details-card {
  display: grid;
  gap: 3px;
}

.details-list {
  overflow: hidden;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
}

.details-list > div {
  display: grid;
  grid-template-columns: minmax(130px, 0.7fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 9px 11px;
  border-bottom: 1px solid var(--border-glass);
  font-size: 0.68rem;
}

.details-list > div:last-child {
  border-bottom: 0;
}

.details-list span {
  color: var(--text-muted);
}

.details-list strong {
  overflow: hidden;
  font: 0.68rem var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.details-list button {
  border: 0;
  background: none;
  color: var(--primary);
  cursor: pointer;
  font: inherit;
  font-size: 0.62rem;
}

@media (max-width: 820px) {
  .transfer-grid {
    grid-template-columns: 1fr;
  }

  .input-card,
  .output-card {
    min-height: 0;
  }

  .editor-wrap textarea {
    height: 280px;
  }
}

@media (max-width: 520px) {
  .tool-intro {
    align-items: flex-start;
    flex-direction: column;
  }

  .direction-switch {
    width: 100%;
  }

  .direction-switch button {
    flex: 1;
    padding-inline: 6px;
  }

  .card-heading {
    flex-direction: column;
  }

  .card-actions {
    justify-content: flex-start;
  }

  .output-meta {
    grid-template-columns: 1fr;
  }

  .qr-box {
    justify-items: center;
  }

  .details-list > div {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
