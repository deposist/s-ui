<template>
  <v-container fluid class="migrate-xui-schedule">
    <v-row>
      <v-col cols="12" md="6">
        <div class="text-h5">{{ $t('migrateXui.schedule.title') }}</div>
      </v-col>
      <v-spacer></v-spacer>
      <v-col cols="12" md="auto">
        <v-btn variant="tonal" prepend-icon="mdi-arrow-left" @click="$router.push('/migrate-xui')">
          {{ $t('migrateXui.back') }}
        </v-btn>
      </v-col>
    </v-row>

    <v-alert
      v-if="remoteDisabled"
      type="warning"
      variant="tonal"
      class="mb-4"
      :title="$t('migrateXui.schedule.remoteDisabled')"
    ></v-alert>

    <v-row>
      <v-col cols="12" lg="5">
        <v-card class="pa-4" rounded="lg">
          <v-row>
            <v-col cols="12" sm="6">
              <v-text-field v-model.trim="form.name" :label="$t('migrateXui.schedule.profileName')" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6">
              <v-select v-model="form.source.type" :items="sourceTypes" :label="$t('migrateXui.schedule.sourceType')" hide-details></v-select>
            </v-col>
            <v-col cols="12">
              <v-text-field v-model.trim="form.source.url" :label="sourceURLLabel" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model.trim="form.source.username" :label="$t('migrateXui.schedule.username')" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field
                v-model="form.source.password"
                :label="$t('migrateXui.schedule.password')"
                type="password"
                hide-details
              ></v-text-field>
            </v-col>
            <v-col v-if="form.source.type === 'ssh'" cols="12" sm="6">
              <v-text-field v-model.trim="form.source.keyPath" :label="$t('migrateXui.schedule.keyPath')" hide-details></v-text-field>
            </v-col>
            <v-col v-if="form.source.type === 'ssh'" cols="12" sm="6">
              <v-text-field v-model.trim="form.source.remotePath" :label="$t('migrateXui.schedule.remotePath')" hide-details></v-text-field>
            </v-col>
            <v-col v-if="form.source.type === 'ssh'" cols="12" sm="6">
              <v-checkbox v-model="form.source.confirmHostKey" :label="$t('migrateXui.schedule.acceptHostKey')" hide-details></v-checkbox>
            </v-col>
            <v-col v-if="form.source.type === 'ssh'" cols="12" sm="6">
              <v-text-field v-model.trim="form.source.hostKeyFingerprint" :label="$t('migrateXui.schedule.hostFingerprint')" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6">
              <v-select v-model="form.strategy" :items="strategyItems" :label="$t('migrateXui.strategy')" hide-details></v-select>
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model.trim="form.schedule" :label="$t('migrateXui.schedule.cron')" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6">
              <v-checkbox v-model="form.onlyNew" :label="$t('migrateXui.schedule.onlyNew')" hide-details></v-checkbox>
            </v-col>
            <v-col cols="12" sm="6">
              <v-checkbox v-model="form.enabled" :label="$t('enable')" hide-details></v-checkbox>
            </v-col>
            <v-col cols="12">
              <v-btn color="primary" prepend-icon="mdi-content-save" :loading="saving" :disabled="remoteDisabled" @click="saveProfile">
                {{ $t('actions.save') }}
              </v-btn>
            </v-col>
          </v-row>
        </v-card>
      </v-col>

      <v-col cols="12" lg="7">
        <v-card class="pa-4" rounded="lg">
          <v-data-table
            density="compact"
            :headers="headers"
            :items="profiles"
            :loading="loading"
            item-value="id"
          >
            <template #item.enabled="{ item }">
              <v-chip size="small" :color="rowItem(item).enabled ? 'success' : undefined" variant="tonal">
                {{ rowItem(item).enabled ? $t('enable') : $t('disable') }}
              </v-chip>
            </template>
            <template #item.lastRunAt="{ item }">
              {{ formatDate(rowItem(item).lastRunAt) }}
            </template>
            <template #item.actions="{ item }">
              <v-btn
                icon="mdi-play"
                variant="text"
                :disabled="remoteDisabled || runningId === rowItem(item).id"
                :loading="runningId === rowItem(item).id"
                @click="runNow(rowItem(item))"
              >
                <v-icon icon="mdi-play"></v-icon>
                <v-tooltip activator="parent" location="top" :text="$t('migrateXui.schedule.runNow')"></v-tooltip>
              </v-btn>
              <v-btn icon="mdi-pause" variant="text" :disabled="!rowItem(item).enabled" @click="disableProfile(rowItem(item))">
                <v-icon icon="mdi-pause"></v-icon>
                <v-tooltip activator="parent" location="top" :text="$t('disable')"></v-tooltip>
              </v-btn>
            </template>
          </v-data-table>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'

export default {
  data() {
    return {
      profiles: [] as any[],
      loading: false,
      saving: false,
      runningId: null as number | null,
      remoteDisabled: false,
      form: {
        name: '',
        sourceType: 'ssh',
        strategy: 'merge',
        onlyNew: true,
        enabled: true,
        schedule: '0 */6 * * *',
        source: {
          type: 'ssh',
          url: '',
          username: '',
          password: '',
          keyPath: '',
          remotePath: '/etc/x-ui/x-ui.db',
          confirmHostKey: false,
          hostKeyFingerprint: '',
          baseUrl: '',
        },
      },
    }
  },
  computed: {
    sourceTypes(): any[] {
      return ['ssh', 'xuihttp'].map(value => ({ value, title: this.$t(`migrateXui.schedule.sources.${value}`) }))
    },
    strategyItems(): any[] {
      return ['merge', 'replace', 'skip'].map(value => ({ value, title: this.$t(`migrateXui.actions.${value}`) }))
    },
    sourceURLLabel(): string {
      return this.form.source.type === 'ssh'
        ? this.$t('migrateXui.schedule.sshUrl') as string
        : this.$t('migrateXui.schedule.httpUrl') as string
    },
    headers(): any[] {
      return [
        { title: 'ID', key: 'id', width: 80 },
        { title: this.$t('migrateXui.schedule.profileName'), key: 'name' },
        { title: this.$t('migrateXui.schedule.sourceType'), key: 'sourceType' },
        { title: this.$t('migrateXui.strategy'), key: 'strategy' },
        { title: this.$t('enable'), key: 'enabled', width: 120 },
        { title: this.$t('migrateXui.schedule.lastRun'), key: 'lastRunAt' },
        { title: this.$t('migrateXui.schedule.status'), key: 'lastRunStatus' },
        { title: this.$t('actions.action'), key: 'actions', sortable: false, width: 120 },
      ]
    },
  },
  watch: {
    'form.source.type'(value: string) {
      this.form.sourceType = value
      this.form.source.baseUrl = value === 'xuihttp' ? this.form.source.url : ''
    },
    'form.source.url'(value: string) {
      if (this.form.source.type === 'xuihttp') this.form.source.baseUrl = value
    },
  },
  mounted() {
    this.loadStatus()
    this.loadProfiles()
  },
  methods: {
    async loadStatus() {
      const msg = await HttpUtils.get('api/import-xui/remote/status')
      if (msg.success) this.remoteDisabled = Boolean(msg.obj?.disabled)
    },
    async loadProfiles() {
      this.loading = true
      const msg = await HttpUtils.get('api/import-xui/sync/profiles')
      this.loading = false
      if (msg.success) this.profiles = msg.obj ?? []
    },
    async saveProfile() {
      this.saving = true
      const payload = {
        ...this.form,
        sourceType: this.form.source.type,
        source: {
          ...this.form.source,
          baseUrl: this.form.source.type === 'xuihttp' ? this.form.source.url : this.form.source.baseUrl,
        },
      }
      const msg = await HttpUtils.post('api/import-xui/sync/profiles', payload)
      this.saving = false
      if (msg.success) {
        this.form.name = ''
        await this.loadProfiles()
      }
    },
    async runNow(profile: any) {
      this.runningId = profile.id
      const body = new URLSearchParams()
      body.set('id', String(profile.id))
      const msg = await HttpUtils.post('api/import-xui/sync/run', body)
      this.runningId = null
      if (msg.success) await this.loadProfiles()
    },
    async disableProfile(profile: any) {
      const body = new URLSearchParams()
      body.set('id', String(profile.id))
      const msg = await HttpUtils.post('api/import-xui/sync/disable', body)
      if (msg.success) await this.loadProfiles()
    },
    rowItem(item: any): any {
      return item?.raw ?? item
    },
    formatDate(value: number): string {
      if (!value) return '-'
      return new Date(value * 1000).toLocaleString()
    },
  },
}
</script>

<style scoped>
.migrate-xui-schedule {
  max-width: 1440px;
}
</style>
