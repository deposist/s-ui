<template>
  <v-card :loading="loading">
    <v-card-title>{{ $t('telegram.title') }}</v-card-title>
    <v-divider></v-divider>
    <v-card-text>
      <v-alert type="warning" variant="tonal" density="compact" class="mb-4">
        {{ $t('telegram.securityWarning') }}
      </v-alert>
      <v-row align="center">
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" v-model="telegramEnabled" :label="$t('telegram.enabled')" hide-details />
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" v-model="telegramNotifyCpu" :label="$t('telegram.notifyCpu')" hide-details />
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" v-model="telegramReport" :label="$t('telegram.report')" hide-details />
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" sm="6" md="4">
          <SettingsSecretField
            v-model="settings.telegramBotToken"
            :has-secret="settings.telegramBotTokenHasSecret"
            :label="$t('telegram.botToken')"
            hide-details
          />
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-text-field v-model="settings.telegramChatID" :label="$t('telegram.chatId')" hide-details />
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-text-field
            v-model.number="telegramCpuThreshold"
            type="number"
            min="1"
            max="100"
            :label="$t('telegram.cpuThreshold')"
            suffix="%"
            hide-details
          />
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" sm="6" md="4">
          <SettingsSecretField
            v-model="settings.telegramProxyURL"
            :has-secret="settings.telegramProxyURLHasSecret"
            :label="$t('telegram.proxyUrl')"
            hide-details
          />
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <SettingsSecretField
            v-model="settings.telegramProxyUsername"
            :has-secret="settings.telegramProxyUsernameHasSecret"
            :label="$t('telegram.proxyUsername')"
            hide-details
          />
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <SettingsSecretField
            v-model="settings.telegramProxyPassword"
            :has-secret="settings.telegramProxyPasswordHasSecret"
            :label="$t('telegram.proxyPassword')"
            hide-details
          />
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" md="8">
          <v-text-field v-model="settings.telegramReportCron" :label="$t('telegram.reportCron')" hide-details />
        </v-col>
      </v-row>
      <v-row align="center">
        <v-col cols="auto">
          <v-btn color="primary" :loading="loading" :disabled="!stateChange" @click="save">
            {{ $t('actions.save') }}
          </v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn variant="outlined" color="primary" :loading="testLoading" @click="testTelegram">
            <v-icon icon="mdi-send-check-outline" class="me-2" />
            {{ $t('actions.test') }}
          </v-btn>
        </v-col>
        <v-col cols="12" md="6" v-if="testResult">
          <v-chip :color="testResult.success ? 'success' : 'warning'" label>
            {{ testResult.success ? $t('success') : testResult.errorClass }}
          </v-chip>
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import { push } from 'notivue'
import SettingsSecretField from '@/components/SettingsSecretField.vue'
import { normalizeSecretFields, stripSecretPlaceholders } from '@/components/settingsSecretField'

type TelegramSettingsMap = Record<string, string>

type TelegramResult = {
  success: boolean
  errorClass?: string
}

const telegramSettingKeys = [
  'telegramEnabled',
  'telegramBotToken',
  'telegramChatID',
  'telegramProxyURL',
  'telegramProxyUsername',
  'telegramProxyPassword',
  'telegramCpuThreshold',
  'telegramNotifyCpu',
  'telegramReport',
  'telegramReportCron',
]

const defaultTelegramSettings: TelegramSettingsMap = {
  telegramEnabled: 'false',
  telegramBotToken: '',
  telegramBotTokenHasSecret: 'false',
  telegramChatID: '',
  telegramProxyURL: '',
  telegramProxyURLHasSecret: 'false',
  telegramProxyUsername: '',
  telegramProxyUsernameHasSecret: 'false',
  telegramProxyPassword: '',
  telegramProxyPasswordHasSecret: 'false',
  telegramCpuThreshold: '90',
  telegramNotifyCpu: 'false',
  telegramReport: 'false',
  telegramReportCron: '',
}

const loading = ref(false)
const testLoading = ref(false)
const settings = ref<TelegramSettingsMap>({ ...defaultTelegramSettings })
const oldSettings = ref<TelegramSettingsMap>({ ...defaultTelegramSettings })
const testResult = ref<TelegramResult | null>(null)

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/settings')
  if (msg.success) {
    setData(msg.obj ?? {})
  }
  loading.value = false
}

onMounted(loadData)

const setData = (data: TelegramSettingsMap) => {
  const normalized = normalizeSecretFields({ ...defaultTelegramSettings, ...data })
  settings.value = pickTelegramSettings(normalized)
  oldSettings.value = { ...settings.value }
}

const pickTelegramSettings = (source: TelegramSettingsMap): TelegramSettingsMap => {
  const picked: TelegramSettingsMap = {}
  for (const key of telegramSettingKeys) {
    picked[key] = String(source[key] ?? '')
    picked[key + 'HasSecret'] = String(source[key + 'HasSecret'] ?? 'false')
  }
  return picked
}

const boolSetting = (key: string) => computed({
  get: () => settings.value[key] === 'true',
  set: (value: boolean) => { settings.value[key] = value ? 'true' : 'false' },
})

const telegramEnabled = boolSetting('telegramEnabled')
const telegramNotifyCpu = boolSetting('telegramNotifyCpu')
const telegramReport = boolSetting('telegramReport')

const telegramCpuThreshold = computed({
  get: () => Number(settings.value.telegramCpuThreshold || 90),
  set: (value: number) => {
    const normalized = Number.isFinite(value) && value > 0 ? Math.min(Math.trunc(value), 100) : 90
    settings.value.telegramCpuThreshold = normalized.toString()
  },
})

const save = async () => {
  loading.value = true
  const payload = stripSecretPlaceholders(pickTelegramSettings(settings.value))
  const msg = await HttpUtils.post('api/save', { object: 'settings', action: 'set', data: JSON.stringify(payload) })
  if (msg.success) {
    push.success({
      title: i18n.global.t('success'),
      duration: 5000,
      message: i18n.global.t('actions.set') + ' ' + i18n.global.t('telegram.title'),
    })
    setData(msg.obj.settings)
  }
  loading.value = false
}

const testTelegram = async () => {
  testLoading.value = true
  testResult.value = null
  const msg = await HttpUtils.post('api/telegram/test', {})
  if (msg.success) {
    testResult.value = msg.obj as TelegramResult
  }
  testLoading.value = false
}

const stateChange = computed(() => {
  return !FindDiff.deepCompare(settings.value, oldSettings.value)
})
</script>
