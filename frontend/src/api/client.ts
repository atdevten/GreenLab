import axios from 'axios'

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export const client = axios.create({ baseURL: BASE_URL, timeout: 10000 })

client.interceptors.request.use(config => {
  const token = localStorage.getItem('access_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

client.interceptors.response.use(
  r => {
    // Unwrap the backend envelope: { success, data } → response.data = data
    if (r.data && typeof r.data === 'object' && 'success' in r.data && 'data' in r.data) {
      r.data = r.data.data
    }
    return r
  },
  async err => {
    const original = err.config
    if (err.response?.status === 401 && !original._retry) {
      original._retry = true
      const refresh = localStorage.getItem('refresh_token')
      if (refresh) {
        try {
          const { data } = await axios.post(`${BASE_URL}/api/v1/auth/refresh`, { refresh_token: refresh })
          const token = data?.data?.access_token ?? data.access_token
          localStorage.setItem('access_token', token)
          if (data?.data?.refresh_token) localStorage.setItem('refresh_token', data.data.refresh_token)
          original.headers.Authorization = `Bearer ${token}`
          return client(original)
        } catch {
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          window.location.href = '/login'
        }
      } else {
        window.location.href = '/login'
      }
    }
    return Promise.reject(err)
  }
)
