#!/usr/bin/env bash
# 交互提示 helpers

# ask VAR "提示词" "默认值"
# 空回车 = 用默认值
ask() {
    local var="$1" prompt="$2" default="${3:-}"
    local input
    if [ -n "$default" ]; then
        read -rp "$(echo -e "${CYAN}?${RESET} ${prompt} ${DIM}[$default]${RESET}: ")" input
        input="${input:-$default}"
    else
        read -rp "$(echo -e "${CYAN}?${RESET} ${prompt}: ")" input
    fi
    printf -v "$var" '%s' "$input"
    export "$var"
}

# ask_password VAR "提示词" —— 不回显
ask_password() {
    local var="$1" prompt="$2"
    local input
    read -rsp "$(echo -e "${CYAN}?${RESET} ${prompt}: ")" input
    echo
    printf -v "$var" '%s' "$input"
    export "$var"
}

# ask_secret VAR "提示词" [默认生成随机 32 字符]
ask_secret() {
    local var="$1" prompt="$2"
    local hint="按回车自动生成"
    local input
    read -rsp "$(echo -e "${CYAN}?${RESET} ${prompt} ${DIM}[$hint]${RESET}: ")" input
    echo
    if [ -z "$input" ]; then
        input="$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
        success "已生成随机 ${var}"
    fi
    printf -v "$var" '%s' "$input"
    export "$var"
}

# confirm "提示词" [y|n]
# 返回 0 = yes, 1 = no
confirm() {
    local prompt="$1" default="${2:-n}"
    local hint input
    if [ "$default" = "y" ]; then hint="[Y/n]"; else hint="[y/N]"; fi
    read -rp "$(echo -e "${YELLOW}?${RESET} ${prompt} ${DIM}${hint}${RESET} ")" input
    input="${input:-$default}"
    case "$input" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) return 1 ;;
    esac
}

# select_one VAR "提示词" "选项1" "选项2" ...
select_one() {
    local var="$1" prompt="$2"; shift 2
    local options=("$@")
    local i=1
    echo -e "${CYAN}?${RESET} ${prompt}"
    for opt in "${options[@]}"; do
        echo -e "  ${DIM}$i)${RESET} $opt"
        ((i++))
    done
    local choice
    while true; do
        read -rp "$(echo -e "${CYAN}>${RESET} 输入编号 [1-${#options[@]}]: ")" choice
        if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le ${#options[@]} ]; then
            printf -v "$var" '%s' "${options[$((choice-1))]}"
            export "$var"
            return 0
        fi
        error "无效选择"
    done
}

# 交互式暂停（按回车继续）
pause() {
    read -rp "$(echo -e "${DIM}按回车继续...${RESET}")" _
}
