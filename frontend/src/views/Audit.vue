<template>
  <v-card :loading="loading">
    <v-card-title>{{ $t('audit.title') }}</v-card-title>
    <v-divider></v-divider>
    <v-card-text>
      <v-row align="center">
        <v-col cols="12" sm="4" md="3">
          <v-text-field
            v-model.trim="eventFilter"
            :label="$t('audit.event')"
            maxlength="64"
            hide-details
            @keyup.enter="resetAndLoad"
          />
        </v-col>
        <v-col cols="12" sm="4" md="3">
          <v-select
            v-model="severityFilter"
            :label="$t('audit.severity')"
            :items="severityItems"
            hide-details
          />
        </v-col>
        <v-col cols="6" sm="2" md="2">
          <v-select
            v-model.number="limit"
            :label="$t('count')"
            :items="[25, 50, 100, 200]"
            hide-details
          />
        </v-col>
        <v-col cols="auto">
          <v-btn icon="mdi-refresh" variant="tonal" :loading="loading" @click="resetAndLoad">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.update')" />
          </v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn icon="mdi-filter-remove-outline" variant="text" @click="clearFilters">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')" />
          </v-btn>
        </v-col>
      </v-row>
      <v-data-table
        :headers="headers"
        :items="events"
        item-value="id"
        density="compact"
        show-expand
        :items-per-page="limit"
        :hide-default-footer="true"
      >
        <template v-slot:item.dateTime="{ value }">
          <v-chip variant="text" dir="ltr" density="compact">
            {{ dateFormatted(value) }}
          </v-chip>
        </template>
        <template v-slot:item.severity="{ value }">
          <v-chip density="compact" :color="value === 'warn' ? 'warning' : 'primary'" label>
            {{ value }}
          </v-chip>
        </template>
        <template v-slot:expanded-row="{ columns, item }">
          <tr>
            <td :colspan="columns.length">
              <pre dir="ltr" class="audit-details">{{ formatDetails(item.details) }}</pre>
            </td>
          </tr>
        </template>
      </v-data-table>
      <v-row class="mt-2" align="center" justify="end">
        <v-col cols="auto">
          <v-btn icon="mdi-chevron-left" variant="text" :disabled="cursorStack.length === 0 || loading" @click="previousPage">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('audit.previous')" />
          </v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn icon="mdi-chevron-right" variant="text" :disabled="nextCursor === 0 || loading" @click="nextPage">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('audit.next')" />
          </v-btn>
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>

<script lang="ts" setup>
import { i18n, locale } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { onMounted, ref } from 'vue'

type AuditEvent = {
  id: number
  dateTime: number
  actor: string
  event: string
  resource: string
  severity: string
  ip: string
  userAgent: string
  details: unknown
}

const loading = ref(false)
const events = ref<AuditEvent[]>([])
const eventFilter = ref('')
const severityFilter = ref('')
const limit = ref(50)
const currentCursor = ref(0)
const nextCursor = ref(0)
const cursorStack = ref<number[]>([])

const severityItems = [
  { title: i18n.global.t('all'), value: '' },
  { title: 'info', value: 'info' },
  { title: 'warn', value: 'warn' },
]

const headers = [
  { title: 'ID', key: 'id' },
  { title: i18n.global.t('admin.date') + '-' + i18n.global.t('admin.time'), key: 'dateTime' },
  { title: i18n.global.t('admin.actor'), key: 'actor' },
  { title: i18n.global.t('audit.event'), key: 'event' },
  { title: i18n.global.t('audit.severity'), key: 'severity' },
  { title: i18n.global.t('audit.resource'), key: 'resource' },
]

onMounted(() => {
  loadData()
})

const loadData = async () => {
  loading.value = true
  const query: Record<string, string | number> = { limit: limit.value }
  if (currentCursor.value > 0) query.cursor = currentCursor.value
  if (eventFilter.value) query.event = eventFilter.value
  if (severityFilter.value) query.severity = severityFilter.value
  const msg = await HttpUtils.get('api/security/audit', query)
  if (msg.success) {
    events.value = msg.obj?.events ?? []
    nextCursor.value = Number(msg.obj?.nextCursor ?? 0)
  }
  loading.value = false
}

const resetAndLoad = () => {
  currentCursor.value = 0
  cursorStack.value = []
  loadData()
}

const clearFilters = () => {
  eventFilter.value = ''
  severityFilter.value = ''
  resetAndLoad()
}

const nextPage = () => {
  if (nextCursor.value === 0) return
  cursorStack.value.push(currentCursor.value)
  currentCursor.value = nextCursor.value
  loadData()
}

const previousPage = () => {
  const previous = cursorStack.value.pop()
  if (previous === undefined) return
  currentCursor.value = previous
  loadData()
}

const dateFormatted = (value: number): string => {
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString(locale)
}

const formatDetails = (details: unknown): string => {
  if (details == null || details === '') return ''
  if (typeof details === 'string') {
    try {
      return JSON.stringify(JSON.parse(details), null, 2)
    } catch {
      return details
    }
  }
  return JSON.stringify(details, null, 2)
}
</script>

<style scoped>
.audit-details {
  white-space: pre-wrap;
  margin: 8px 0;
}
</style>
