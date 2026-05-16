import axios from 'axios'

let csrfToken: string | null = null
let csrfTokenPromise: Promise<string> | null = null

export const getCSRFToken = async () => {
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

export const clearCSRFToken = () => {
  csrfToken = null
}
