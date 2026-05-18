<template>
  <v-dialog transition="dialog-bottom-transition" width="90%" max-width="500">
    <v-card class="rounded-lg">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('main.backup.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto">
            <v-icon icon="mdi-close" @click="control.visible = false" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclStats')" value="stats" hide-details></v-checkbox>
          </v-col>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclChanges')" value="changes" hide-details></v-checkbox>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="backup()" hide-details>{{ $t('main.backup.backup') }}</v-btn>
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="restore()" hide-details>{{ $t('main.backup.restore') }}</v-btn>
          </v-col>
        </v-row>
        <v-row>
          <v-divider></v-divider>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="config()" hide-details>{{ $t('main.backup.sbConfig') }}</v-btn>
          </v-col>
        </v-row>
        <v-row>
          <v-divider class="mb-2"></v-divider>
          <v-col cols="12">
            <div class="text-subtitle-1 mb-2">{{ $t('main.backup.xui.title') }}</div>
          </v-col>
          <v-col cols="auto">
            <v-checkbox v-model="xuiDryRun" :label="$t('main.backup.xui.dryRun')" hide-details></v-checkbox>
          </v-col>
          <v-col cols="12" sm="auto">
            <v-select
              v-model="xuiStrategy"
              :items="['merge', 'replace', 'skip']"
              :label="$t('main.backup.xui.strategy')"
              density="compact"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12" sm="auto" align-self="center">
            <v-btn color="primary" @click="migrateXui()" hide-details>{{ $t('main.backup.xui.button') }}</v-btn>
          </v-col>
          <v-col cols="12" sm="auto" align-self="center">
            <v-btn variant="tonal" prepend-icon="mdi-open-in-new" @click="openFullXuiMigration()" hide-details>
              {{ $t('main.backup.xui.openFull') }}
            </v-btn>
          </v-col>
          <v-col v-if="xuiReport" cols="12">
            <v-card variant="outlined" class="pa-3">
              <div class="text-subtitle-2 mb-1">{{ $t('main.backup.xui.summary') }}</div>
              <pre class="text-caption xui-report">{{ JSON.stringify(xuiReport.summary, null, 2) }}</pre>
              <template v-if="xuiReport.warnings && xuiReport.warnings.length">
                <div class="text-subtitle-2 mt-2">{{ $t('main.backup.xui.warnings') }}</div>
                <ul>
                  <li v-for="(warning, index) in xuiReport.warnings" :key="index">{{ warning }}</li>
                </ul>
              </template>
            </v-card>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'
export default {
  props: ['control', 'visible'],
  data() {
    return {
      exclude: ["stats", "changes"],
      xuiDryRun: true,
      xuiStrategy: 'merge',
      xuiReport: null as null | Record<string, any>,
    }
  },
  methods: {
    backup() {
      const excludeOption = this.exclude.length>0 ? '?exclude=' +this.exclude.join(',') : ''
      window.location.href = 'api/getdb' + excludeOption
    },
    config() {
      window.location.href = 'api/singbox-config'
    },
    restore() {
      const fileInput = document.createElement('input')
      fileInput.type = 'file'
      fileInput.accept = '.db'

      fileInput.addEventListener('change', async (event: Event) => {
        const inputElement = event.target as HTMLInputElement
        const dbFile = inputElement.files ? inputElement.files[0] : null

        if (dbFile) {
          const formData = new FormData()
          formData.append('db', dbFile)

          this.control.visible = false

          const uploadMsg = await HttpUtils.post('api/importdb', formData)

          if (uploadMsg.success) {
            await new Promise(resolve => setTimeout(resolve, 1000))
            location.reload()
          }
        }
    })

    fileInput.click()
    },
    migrateXui() {
      const fileInput = document.createElement('input')
      fileInput.type = 'file'
      fileInput.accept = '.db'

      fileInput.addEventListener('change', async (event: Event) => {
        const inputElement = event.target as HTMLInputElement
        const dbFile = inputElement.files ? inputElement.files[0] : null

        if (dbFile) {
          const formData = new FormData()
          formData.append('db', dbFile)
          formData.append('dryRun', this.xuiDryRun ? '1' : '0')
          formData.append('strategy', this.xuiStrategy)

          const uploadMsg = await HttpUtils.post('api/import-xui', formData)

          if (uploadMsg.success) {
            this.xuiReport = uploadMsg.obj
            if (!this.xuiDryRun) {
              this.control.visible = false
              await new Promise(resolve => setTimeout(resolve, 1000))
              location.reload()
            }
          }
        }
      })

      fileInput.click()
    },
    openFullXuiMigration() {
      this.control.visible = false
      this.$router.push('/migrate-xui')
    },
  },
  watch: {
    visible(v) {
      if (v) {
        this.exclude = ["stats", "changes"]
        this.xuiReport = null
      }
    },
  },
}
</script>

<style scoped>
.xui-report {
  white-space: pre-wrap;
}
</style>
