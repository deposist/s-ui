import axios from 'axios'

const api = axios.create({
    baseURL: './',
    headers: {
        common: {
            'X-Requested-With': 'XMLHttpRequest',
        },
        post: {
            'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
        },
    },
})

const pendingRequests = new Map<string, AbortController>()

const isDedupeMethod = (method?: string) => {
    const m = (method ?? 'get').toLowerCase()
    // Only deduplicate idempotent reads. Mutating requests with identical
    // URLs but different bodies would otherwise cancel each other.
    return m === 'get' || m === 'head' || m === 'options'
}

const requestKey = (config: any) => {
    const params = config.params ? JSON.stringify(config.params) : ''
    return `${config.method}:${config.url}:${params}`
}

api.interceptors.request.use(
    (config) => {
        if (isDedupeMethod(config.method)) {
            const key = requestKey(config)
            if (pendingRequests.has(key)) {
                pendingRequests.get(key)?.abort('Duplicate request cancelled')
            }
            const controller = new AbortController()
            config.signal = controller.signal
            pendingRequests.set(key, controller)
        }

        if (config.data instanceof FormData) {
            delete config.headers['Content-Type']
        }
        return config
    },
    (error) => Promise.reject(error),
)

api.interceptors.response.use(
    (response) => {
        if (isDedupeMethod(response.config.method)) {
            pendingRequests.delete(requestKey(response.config))
        }
        return response
    },
    (error) => {
        if (axios.isCancel(error) || error.code === 'ERR_CANCELED') {
            console.warn(error.message)
        } else if (error.config && isDedupeMethod(error.config.method)) {
            pendingRequests.delete(requestKey(error.config))
        }
        return Promise.reject(error)
    }
)

export default api
