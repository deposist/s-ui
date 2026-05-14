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

const requestKey = (config: any) => {
    const params = config.params ? JSON.stringify(config.params) : ''
    return `${config.method}:${config.url}:${params}`
}

api.interceptors.request.use(
    (config) => {
        const key = requestKey(config)
        if (pendingRequests.has(key)) {
            pendingRequests.get(key)?.abort('Duplicate request cancelled')
        }
        const controller = new AbortController()
        config.signal = controller.signal
        pendingRequests.set(key, controller)

        if (config.data instanceof FormData) {
            delete config.headers['Content-Type']
        }
        return config
    },
    (error) => Promise.reject(error),
)

api.interceptors.response.use(
    (response) => {
        pendingRequests.delete(requestKey(response.config))
        return response
    },
    (error) => {
        if (axios.isCancel(error) || error.code === 'ERR_CANCELED') {
            console.warn(error.message)
        } else if (error.config) {
            pendingRequests.delete(requestKey(error.config))
        }
        return Promise.reject(error)
    }
)

export default api
