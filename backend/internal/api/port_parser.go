package api

import (
	"strings"
)

// parseDockerfilePort 从 Dockerfile 内容中解析 EXPOSE 端口
func parseDockerfilePort(content string) int {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "EXPOSE ") {
			portStr := strings.TrimSpace(strings.TrimPrefix(upper, "EXPOSE "))
			portStr = strings.Split(portStr, "/")[0]
			portStr = strings.Split(portStr, " ")[0]

			var port int
			for _, ch := range portStr {
				if ch >= '0' && ch <= '9' {
					port = port*10 + int(ch-'0')
				}
			}
			if port > 0 && port <= 65535 {
				return port
			}
		}
	}
	return 0
}

// parseComposePort 从 docker-compose.yml 内容中解析端口映射
func parseComposePort(content, serviceName string) (hostPort, containerPort int) {
	if content == "" {
		return 0, 0
	}

	lines := strings.Split(content, "\n")
	inServices := false
	inService := false
	inPorts := false
	serviceIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 计算当前行缩进
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// 检测 services: 块
		if trimmed == "services:" {
			inServices = true
			serviceIndent = -1
			continue
		}

		// 已经在服务内，先检测 ports: 等配置项
		if inService && trimmed == "ports:" {
			inPorts = true
			continue
		}

		// 解析端口
		if inPorts && strings.HasPrefix(trimmed, "-") {
			portStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			portStr = strings.Trim(portStr, "\"'")

			parts := strings.Split(portStr, ":")
			if len(parts) >= 2 {
				hostPortStr := strings.Split(parts[0], "/")[0]
				containerPortStr := strings.Split(parts[1], "/")[0]

				var hp, cp int
				for _, ch := range hostPortStr {
					if ch >= '0' && ch <= '9' {
						hp = hp*10 + int(ch-'0')
					}
				}
				for _, ch := range containerPortStr {
					if ch >= '0' && ch <= '9' {
						cp = cp*10 + int(ch-'0')
					}
				}

				if hp > 0 && hp <= 65535 && cp > 0 && cp <= 65535 {
					return hp, cp
				}
			}
		}

		// 退出 ports 块（遇到非 - 开头的行）
		if inPorts && !strings.HasPrefix(trimmed, "-") {
			inPorts = false
		}

		// 在 services 块内，检测具体服务（服务名行缩进较浅，通常 2 空格）
		if inServices && strings.HasSuffix(trimmed, ":") && indent > 0 && indent <= 4 {
			svcName := strings.TrimSuffix(trimmed, ":")
			// 跳过 docker-compose 保留关键字
			if isComposeKeyword(svcName) {
				continue
			}
			// 如果指定了 serviceName，只匹配该服务；否则匹配第一个服务
			if serviceName == "" || svcName == serviceName {
				inService = true
				serviceIndent = indent
				inPorts = false
				continue
			}
		}

		// 如果已经在某个服务内，检查是否退出该服务（遇到同级或更浅的缩进）
		if inService && serviceIndent >= 0 && indent <= serviceIndent && !strings.HasPrefix(trimmed, "-") {
			if serviceName != "" {
				// 指定了服务名但已退出该服务，停止解析
				break
			}
			// 未指定服务名，继续检查下一个服务
			inService = false
			inPorts = false
		}
	}

	return 0, 0
}

// isComposeKeyword 检查是否是 docker-compose 的保留关键字
func isComposeKeyword(name string) bool {
	keywords := []string{
		"ports", "volumes", "environment", "depends_on", "networks",
		"build", "image", "container_name", "restart", "command",
		"entrypoint", "labels", "expose", "healthcheck", "logging",
		"deploy", "configs", "secrets", "ulimits", "sysctls",
	}
	for _, kw := range keywords {
		if name == kw {
			return true
		}
	}
	return false
}
