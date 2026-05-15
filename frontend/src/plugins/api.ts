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
let csrfToken: string | null = null
let csrfTokenPromise: Promise<string> | null = null

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

const normalizeURL = (url?: string) => (url ?? '').replace(/^\.\//, '').replace(/^\//, '')

const needsCSRFToken = (method?: string, url?: string) => {
    const m = (method ?? 'get').toLowerCase()
    if (!['post', 'put', 'patch', 'delete'].includes(m)) {
        return false
    }
    const normalized = normalizeURL(url)
    return normalized.startsWith('api/') && normalized !== 'api/login'
}

const fetchCSRFToken = async () => {
    if (csrfToken) {
        return csrfToken
    }
    if (!csrfTokenPromise) {
        csrfTokenPromise = axios.get('api/csrf', {
            baseURL: './',
            headers: {
                'X-Requested-With': 'XMLHttpRequest',
            },
        }).then((response) => {
            const token = response.data?.obj?.token
            if (typeof token !== 'string' || token.length === 0) {
                throw new Error('CSRF token was not returned')
            }
            csrfToken = token
            return token
        }).finally(() => {
            csrfTokenPromise = null
        })
    }
    return csrfTokenPromise
}

api.interceptors.request.use(
    async (config) => {
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
        if (needsCSRFToken(config.method, config.url)) {
            config.headers['X-CSRF-Token'] = await fetchCSRFToken()
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
        if (error.response?.status === 403 && error.response?.data?.msg === 'Invalid CSRF token') {
            csrfToken = null
        }
        return Promise.reject(error)
    }
)

export default api
