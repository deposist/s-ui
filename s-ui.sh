#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

function LOGD() {
    echo -e "${yellow}[Отладка] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[Ошибка] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[Инфо] $* ${plain}"
}

[[ $EUID -ne 0 ]] && LOGE "Ошибка: этот скрипт нужно запускать с правами root!\n" && exit 1

if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Не удалось определить систему, обратитесь к автору!" >&2
    exit 1
fi

echo "Текущий дистрибутив системы: $release"

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -p "$1 [по умолчанию $2]: " temp
        if [[ x"${temp}" == x"" ]]; then
            temp=$2
        fi
    else
        read -p "$1 [y/n]： " temp
    fi
    if [[ x"${temp}" == x"y" || x"${temp}" == x"Y" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "Перезапустить службу ${1}" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}Нажмите Enter, чтобы вернуться в главное меню: ${plain}" && read temp
    show_menu
}

install() {
    bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        if [[ $# == 0 ]]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    confirm "Эта функция принудительно переустановит последнюю версию. Данные не будут потеряны. Продолжить?" "n"
    if [[ $? != 0 ]]; then
        LOGE "Отменено"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi
    bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        LOGI "Обновление завершено, панель автоматически перезапущена"
        exit 0
    fi
}

custom_version() {
    echo "Введите версию панели (например, v1.4.1):"
    read panel_version

    if [ -z "$panel_version" ]; then
        echo "Версия панели не может быть пустой. Выход."
    exit 1
    fi

    [[ "${panel_version}" != v* ]] && panel_version="v${panel_version}"

    download_link="https://raw.githubusercontent.com/deposist/s-ui/main/install.sh"

    install_command="bash <(curl -Ls $download_link) $panel_version"

    echo "Скачивание и установка версии панели $panel_version..."
    eval $install_command
}

uninstall() {
    confirm "Вы уверены, что хотите удалить панель?" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi
    systemctl stop s-ui
    systemctl disable s-ui
    rm /etc/systemd/system/s-ui.service -f
    systemctl daemon-reload
    systemctl reset-failed
    rm /etc/s-ui/ -rf
    rm /usr/local/s-ui/ -rf

    echo ""
    echo -e "Удаление завершено. Если нужно удалить этот скрипт, после выхода выполните ${green}rm /usr/local/s-ui -f${plain}."
    echo ""

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

reset_admin() {
    echo "Не рекомендуется устанавливать учетные данные администратора по умолчанию!"
    confirm "Сбросить учетные данные администратора к значениям по умолчанию?" "n"
    if [[ $? == 0 ]]; then
        /usr/local/s-ui/sui admin -reset
    fi
    before_show_menu
}

set_admin() {
    echo "Не рекомендуется использовать слишком сложный текст для учетных данных администратора."
    read -p "Задайте имя пользователя: " config_account
    read -p "Задайте пароль: " config_password
    /usr/local/s-ui/sui admin -username ${config_account} -password ${config_password}
    before_show_menu
}

view_admin() {
    /usr/local/s-ui/sui admin -show
    before_show_menu
}

reset_setting() {
    confirm "Сбросить настройки к значениям по умолчанию?" "n"
    if [[ $? == 0 ]]; then
        /usr/local/s-ui/sui setting -reset
    fi
    before_show_menu
}

set_setting() {
    echo -e "Введите ${yellow}порт панели${plain} (оставьте пустым, чтобы использовать текущее/стандартное значение):"
    read config_port
    echo -e "Введите ${yellow}путь панели${plain} (оставьте пустым, чтобы использовать текущее/стандартное значение):"
    read config_path

    echo -e "Введите ${yellow}порт подписки${plain} (оставьте пустым, чтобы использовать текущее/стандартное значение):"
    read config_subPort
    echo -e "Введите ${yellow}путь подписки${plain} (оставьте пустым, чтобы использовать текущее/стандартное значение):"
    read config_subPath

    echo -e "${yellow}Инициализация, подождите...${plain}"
    params=""
    [ -z "$config_port" ] || params="$params -port $config_port"
    [ -z "$config_path" ] || params="$params -path $config_path"
    [ -z "$config_subPort" ] || params="$params -subPort $config_subPort"
    [ -z "$config_subPath" ] || params="$params -subPath $config_subPath"
    /usr/local/s-ui/sui setting ${params}
    before_show_menu
}

view_setting() {
    /usr/local/s-ui/sui setting -show
    view_uri
    before_show_menu
}

view_uri() {
    info=$(/usr/local/s-ui/sui uri)
    if [[ $? != 0 ]]; then
        LOGE "Не удалось получить текущий URI"
        before_show_menu
    fi
    LOGI "Панель доступна по следующему URL:"
    echo -e "${green}${info}${plain}"
}

start() {
    check_status $1
    if [[ $? == 0 ]]; then
        echo ""
        LOGI -e "${1} уже работает, повторный запуск не нужен; если нужно, выберите перезапуск"
    else
        systemctl start $1
        sleep 2
        check_status $1
        if [[ $? == 0 ]]; then
            LOGI "${1} успешно запущен"
        else
            LOGE "Не удалось запустить ${1}; возможно, запуск занимает больше двух секунд. Проверьте журнал позже"
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

stop() {
    check_status $1
    if [[ $? == 1 ]]; then
        echo ""
        LOGI "${1} уже остановлен, повторная остановка не нужна!"
    else
        systemctl stop $1
        sleep 2
        check_status
        if [[ $? == 1 ]]; then
            LOGI "${1} успешно остановлен"
        else
            LOGE "Не удалось остановить ${1}; возможно, остановка занимает больше двух секунд. Проверьте журнал позже"
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

restart() {
    systemctl restart $1
    sleep 2
    check_status $1
    if [[ $? == 0 ]]; then
        LOGI "${1} успешно перезапущен"
    else
        LOGE "Не удалось перезапустить ${1}; возможно, запуск занимает больше двух секунд. Проверьте журнал позже"
    fi
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

status() {
    systemctl status s-ui -l
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

enable() {
    systemctl enable $1
    if [[ $? == 0 ]]; then
        LOGI "Автозапуск ${1} успешно включен"
    else
        LOGE "Не удалось включить автозапуск ${1}"
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

disable() {
    systemctl disable $1
    if [[ $? == 0 ]]; then
        LOGI "Автозапуск ${1} успешно отключен"
    else
        LOGE "Не удалось отключить автозапуск ${1}"
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

show_log() {
    journalctl -u $1.service -e --no-pager -f
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

update_shell() {
    wget -O /usr/bin/s-ui -N --no-check-certificate https://github.com/deposist/s-ui/raw/main/s-ui.sh
    if [[ $? != 0 ]]; then
        echo ""
        LOGE "Не удалось скачать скрипт. Проверьте, есть ли на сервере доступ к GitHub"
        before_show_menu
    else
        chmod +x /usr/bin/s-ui
        LOGI "Скрипт успешно обновлен, запустите его заново" && exit 0
    fi
}

check_status() {
    if [[ ! -f "/etc/systemd/system/$1.service" ]]; then
        return 2
    fi
    temp=$(systemctl status "$1" | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
    if [[ x"${temp}" == x"running" ]]; then
        return 0
    else
        return 1
    fi
}

check_enabled() {
    temp=$(systemctl is-enabled $1)
    if [[ x"${temp}" == x"enabled" ]]; then
        return 0
    else
        return 1
    fi
}

check_uninstall() {
    check_status s-ui
    if [[ $? != 2 ]]; then
        echo ""
        LOGE "Панель уже установлена, повторная установка не нужна"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status s-ui
    if [[ $? == 2 ]]; then
        echo ""
        LOGE "Сначала установите панель"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status $1
    case $? in
    0)
        echo -e "${1} статус: ${green}работает${plain}"
        show_enable_status $1
        ;;
    1)
        echo -e "${1} статус: ${yellow}не работает${plain}"
        show_enable_status $1
        ;;
    2)
        echo -e "${1} статус: ${red}не установлен${plain}"
        ;;
    esac
}

show_enable_status() {
    check_enabled $1
    if [[ $? == 0 ]]; then
        echo -e "${1} автозапуск: ${green}да${plain}"
    else
        echo -e "${1} автозапуск: ${red}нет${plain}"
    fi
}

check_s-ui_status() {
    count=$(ps -ef | grep "sui" | grep -v "grep" | wc -l)
    if [[ count -ne 0 ]]; then
        return 0
    else
        return 1
    fi
}

show_s-ui_status() {
    check_s-ui_status
    if [[ $? == 0 ]]; then
        echo -e "s-ui статус: ${green}работает${plain}"
    else
        echo -e "s-ui статус: ${red}не работает${plain}"
    fi
}

bbr_menu() {
    echo -e "${green}\t1.${plain} Включить BBR"
    echo -e "${green}\t2.${plain} Отключить BBR"
    echo -e "${green}\t0.${plain} Вернуться в главное меню"
    read -p "Выберите пункт: " choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        enable_bbr
        ;;
    2)
        disable_bbr
        ;;
    *) echo "Недопустимый выбор" ;;
    esac
}

disable_bbr() {
    if ! grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf || ! grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${yellow}BBR сейчас не включен.${plain}"
        exit 0
    fi
    sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/' /etc/sysctl.conf
    sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/' /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "cubic" ]]; then
        echo -e "${green}BBR успешно заменен на CUBIC.${plain}"
    else
        echo -e "${red}Не удалось заменить BBR на CUBIC. Проверьте системную конфигурацию.${plain}"
    fi
}

enable_bbr() {
    if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${green}BBR уже включен!${plain}"
        exit 0
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -yqq --no-install-recommends ca-certificates
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install ca-certificates
        ;;
    fedora)
        dnf -y update && dnf -y install ca-certificates
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm ca-certificates
        ;;
    *)
        echo -e "${red}Операционная система не поддерживается. Проверьте скрипт и установите нужные пакеты вручную.${plain}\n"
        exit 1
        ;;
    esac
    echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "bbr" ]]; then
        echo -e "${green}BBR успешно включен.${plain}"
    else
        echo -e "${red}Не удалось включить BBR. Проверьте системную конфигурацию.${plain}"
    fi
}

install_acme() {
    cd ~
    LOGI "Установка acme..."
    curl https://get.acme.sh | sh
    if [ $? -ne 0 ]; then
        LOGE "Не удалось установить acme"
        return 1
    else
        LOGI "acme успешно установлен"
    fi
    return 0
}

ssl_cert_issue_main() {
    echo -e "${green}\t1.${plain} Получить SSL"
    echo -e "${green}\t2.${plain} Отозвать сертификат"
    echo -e "${green}\t3.${plain} Принудительно продлить"
    echo -e "${green}\t4.${plain} Самоподписанный сертификат"
    read -p "Выберите пункт: " choice
    case "$choice" in
        1) ssl_cert_issue ;;
        2)
            local domain=""
            read -p "Введите домен сертификата для отзыва: " domain
            ~/.acme.sh/acme.sh --revoke -d ${domain}
            LOGI "Сертификат отозван"
            ;;
        3)
            local domain=""
            read -p "Введите домен SSL-сертификата для принудительного продления: " domain
            ~/.acme.sh/acme.sh --renew -d ${domain} --force ;;
        4)
            generate_self_signed_cert
            ;;
        *) echo "Недопустимый выбор" ;;
    esac
}

ssl_cert_issue() {
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "acme.sh не найден, будет выполнена установка"
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "Не удалось установить acme, проверьте журнал"
            exit 1
        fi
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt update && apt install socat -y
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install socat
        ;;
    fedora)
        dnf -y update && dnf -y install socat
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat
        ;;
    *)
        echo -e "${red}Операционная система не поддерживается. Проверьте скрипт и установите нужные пакеты вручную.${plain}\n"
        exit 1
        ;;
    esac
    if [ $? -ne 0 ]; then
        LOGE "Не удалось установить socat, проверьте журнал"
        exit 1
    else
        LOGI "socat успешно установлен..."
    fi

    local domain=""
    read -p "Введите ваш домен: " domain
    LOGD "Ваш домен: ${domain}, выполняется проверка..."
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')

    if [ ${currentCert} == ${domain} ]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "Сертификат уже существует в системе, повторный выпуск невозможен. Текущие данные сертификата:"
        LOGI "$certInfo"
        exit 1
    else
        LOGI "Домен готов к выпуску сертификата..."
    fi

    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    local WebPort=80
    read -p "Выберите порт, по умолчанию используется 80: " WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        LOGE "Введенный порт ${WebPort} недопустим, будет использован порт по умолчанию"
    fi
    LOGI "Для выпуска сертификата будет использован порт ${WebPort}. Убедитесь, что он открыт..."
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    ~/.acme.sh/acme.sh --issue -d ${domain} --standalone --httpport ${WebPort}
    if [ $? -ne 0 ]; then
        LOGE "Не удалось выпустить сертификат, проверьте журнал"
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGE "Сертификат успешно выпущен, установка сертификата..."
    fi
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem

    if [ $? -ne 0 ]; then
        LOGE "Не удалось установить сертификат, выход"
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGI "Сертификат успешно установлен, включается автоматическое продление..."
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "Не удалось включить автоматическое продление. Данные сертификата:"
        ls -lah cert/*
        chmod 755 $certPath/*
        exit 1
    else
        LOGI "Автоматическое продление включено. Данные сертификата:"
        ls -lah cert/*
        chmod 755 $certPath/*
    fi
}

ssl_cert_issue_CF() {
    echo -E ""
    LOGD "******Инструкция******"
    echo "1) Запросить новый сертификат через Cloudflare"
    echo "2) Принудительно продлить существующий сертификат"
    echo "3) Вернуться в меню"
    read -p "Введите ваш выбор [1-3]: " choice

    certPath="/root/cert-CF"

    case $choice in
        1|2)
            force_flag=""
            if [ "$choice" -eq 2 ]; then
                force_flag="--force"
                echo "Принудительный повторный выпуск SSL-сертификата..."
            else
                echo "Начинается выпуск SSL-сертификата..."
            fi

            LOGD "******Инструкция******"
            LOGI "Этому Acme-скрипту нужны следующие данные:"
            LOGI "1. Email учетной записи Cloudflare"
            LOGI "2. Глобальный API Key Cloudflare"
            LOGI "3. Домен, DNS которого через Cloudflare указывает на текущий сервер"
            LOGI "4. Скрипт запросит сертификат; путь установки по умолчанию: /root/cert"
            confirm "Подтвердить? [y/n]" "y"
            if [ $? -eq 0 ]; then
                if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                    echo "acme.sh не найден. Установка..."
                    install_acme
                    if [ $? -ne 0 ]; then
                        LOGE "Не удалось установить acme, проверьте журнал"
                        exit 1
                    fi
                fi

                CF_Domain=""
                if [ ! -d "$certPath" ]; then
                    mkdir -p $certPath
                else
                    rm -rf $certPath
                    mkdir -p $certPath
                fi

                LOGD "Укажите домен:"
                read -p "Введите домен: " CF_Domain
                LOGD "Ваш домен установлен: ${CF_Domain}"

                CF_GlobalKey=""
                CF_AccountEmail=""
                LOGD "Укажите API key:"
                read -p "Введите key: " CF_GlobalKey
                LOGD "Ваш API key: ${CF_GlobalKey}"

                LOGD "Укажите email учетной записи:"
                read -p "Введите email: " CF_AccountEmail
                LOGD "Email учетной записи: ${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                if [ $? -ne 0 ]; then
                    LOGE "Не удалось установить Let's Encrypt как CA по умолчанию, выход..."
                    exit 1
                fi

                export CF_Key="${CF_GlobalKey}"
                export CF_Email="${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --issue --dns dns_cf -d ${CF_Domain} -d *.${CF_Domain} $force_flag --log
                if [ $? -ne 0 ]; then
                    LOGE "Не удалось выпустить сертификат, выход..."
                    exit 1
                else
                    LOGI "Сертификат успешно выпущен, установка..."
                fi

                mkdir -p ${certPath}/${CF_Domain}
                if [ $? -ne 0 ]; then
                    LOGE "Не удалось создать каталог: ${certPath}/${CF_Domain}"
                    exit 1
                fi

                ~/.acme.sh/acme.sh --installcert -d ${CF_Domain} -d *.${CF_Domain} \
                    --fullchain-file ${certPath}/${CF_Domain}/fullchain.pem \
                    --key-file ${certPath}/${CF_Domain}/privkey.pem

                if [ $? -ne 0 ]; then
                    LOGE "Не удалось установить сертификат, выход..."
                    exit 1
                else
                    LOGI "Сертификат успешно установлен, включается автоматическое обновление..."
                fi

                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                if [ $? -ne 0 ]; then
                    LOGE "Не удалось настроить автоматическое обновление, выход..."
                    exit 1
                else
                    LOGI "Сертификат установлен, автоматическое продление включено."
                    ls -lah ${certPath}/${CF_Domain}
                    chmod 755 ${certPath}/${CF_Domain}
                fi
            fi
            show_menu
            ;;
        3)
            echo "Выход..."
            show_menu
            ;;
        *)
            echo "Недопустимый выбор, попробуйте снова."
            show_menu
            ;;
    esac
}

generate_self_signed_cert() {
    cert_dir="/etc/sing-box"
    mkdir -p "$cert_dir"
    LOGI "Выберите тип сертификата:"
    echo -e "${green}\t1.${plain} Ed25519 (рекомендуется)"
    echo -e "${green}\t2.${plain} RSA 2048"
    echo -e "${green}\t3.${plain} RSA 4096"
    echo -e "${green}\t4.${plain} ECDSA prime256v1"
    echo -e "${green}\t5.${plain} ECDSA secp384r1"
    read -p "Введите ваш выбор [1-5, по умолчанию 1]: " cert_type
    cert_type=${cert_type:-1}

    case "$cert_type" in
        1)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
        2)
            algo="rsa"
            key_opt="-newkey rsa:2048"
            ;;
        3)
            algo="rsa"
            key_opt="-newkey rsa:4096"
            ;;
        4)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:prime256v1"
            ;;
        5)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:secp384r1"
            ;;
        *)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
    esac

    LOGI "Генерация самоподписанного сертификата ($algo)..."
    sudo openssl req -x509 -nodes -days 3650 $key_opt \
        -keyout "${cert_dir}/self.key" \
        -out "${cert_dir}/self.crt" \
        -subj "/CN=myserver"
    if [[ $? -eq 0 ]]; then
        sudo chmod 600 "${cert_dir}/self."*
        LOGI "Самоподписанный сертификат успешно создан!"
        LOGI "Путь сертификата: ${cert_dir}/self.crt"
        LOGI "Путь ключа: ${cert_dir}/self.key"
    else
        LOGE "Не удалось создать самоподписанный сертификат."
    fi
    before_show_menu
}

show_usage() {
    echo -e "Использование меню управления S-UI"
    echo -e "------------------------------------------"
    echo -e "Подкоманды:"
    echo -e "s-ui              - скрипт управления администратора"
    echo -e "s-ui start        - запустить s-ui"
    echo -e "s-ui stop         - остановить s-ui"
    echo -e "s-ui restart      - перезапустить s-ui"
    echo -e "s-ui status       - показать текущий статус s-ui"
    echo -e "s-ui enable       - включить автозапуск"
    echo -e "s-ui disable      - отключить автозапуск"
    echo -e "s-ui log          - показать журнал s-ui"
    echo -e "s-ui update       - обновить"
    echo -e "s-ui install      - установить"
    echo -e "s-ui uninstall    - удалить"
    echo -e "s-ui help         - справка по меню управления"
    echo -e "------------------------------------------"
}

show_menu() {
  echo -e "
  ${green}Скрипт управления S-UI ${plain}
---------------------------------------------------------------
  ${green}0.${plain} Выход
---------------------------------------------------------------
  ${green}1.${plain} Установить
  ${green}2.${plain} Обновить
  ${green}3.${plain} Пользовательская версия
  ${green}4.${plain} Удалить
---------------------------------------------------------------
  ${green}5.${plain} Сбросить учетные данные администратора по умолчанию
  ${green}6.${plain} Задать учетные данные администратора
  ${green}7.${plain} Показать учетные данные администратора
---------------------------------------------------------------
  ${green}8.${plain} Сбросить настройки панели
  ${green}9.${plain} Настроить панель
  ${green}10.${plain} Показать настройки панели
---------------------------------------------------------------
  ${green}11.${plain} Запустить S-UI
  ${green}12.${plain} Остановить S-UI
  ${green}13.${plain} Перезапустить S-UI
  ${green}14.${plain} Показать статус S-UI
  ${green}15.${plain} Показать журнал S-UI
  ${green}16.${plain} Включить автозапуск S-UI
  ${green}17.${plain} Отключить автозапуск S-UI
---------------------------------------------------------------
  ${green}18.${plain} Включить или отключить BBR
  ${green}19.${plain} Управление SSL-сертификатами
  ${green}20.${plain} SSL-сертификат Cloudflare
---------------------------------------------------------------
 "
    show_status s-ui
    echo && read -p "Введите ваш выбор [0-20]: " num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && custom_version
        ;;
    4)
        check_install && uninstall
        ;;
    5)
        check_install && reset_admin
        ;;
    6)
        check_install && set_admin
        ;;
    7)
        check_install && view_admin
        ;;
    8)
        check_install && reset_setting
        ;;
    9)
        check_install && set_setting
        ;;
    10)
        check_install && view_setting
        ;;
    11)
        check_install && start s-ui
        ;;
    12)
        check_install && stop s-ui
        ;;
    13)
        check_install && restart s-ui
        ;;
    14)
        check_install && status s-ui
        ;;
    15)
        check_install && show_log s-ui
        ;;
    16)
        check_install && enable s-ui
        ;;
    17)
        check_install && disable s-ui
        ;;
    18)
        bbr_menu
        ;;
    19)
        ssl_cert_issue_main
        ;;
    20)
        ssl_cert_issue_CF
        ;;
    *)
        LOGE "Введите корректное число [0-20]"
        ;;
    esac
}

if [[ $# > 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start s-ui 0
        ;;
    "stop")
        check_install 0 && stop s-ui 0
        ;;
    "restart")
        check_install 0 && restart s-ui 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "enable")
        check_install 0 && enable s-ui 0
        ;;
    "disable")
        check_install 0 && disable s-ui 0
        ;;
    "log")
        check_install 0 && show_log s-ui 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
