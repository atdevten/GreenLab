import { client } from './client'
import type { LoginRequest, LoginResponse, SignupRequest, SignupResponse, User } from '../types'

export const authApi = {
  login: (data: LoginRequest) => client.post<LoginResponse>('/api/v1/auth/login', data),
  signup: (data: SignupRequest) => client.post<SignupResponse>('/api/v1/auth/signup', data),
  logout: () => client.post('/api/v1/auth/logout'),
  getMe: () => client.get<User>('/api/v1/auth/me'),
  updateMe: (data: { first_name?: string; last_name?: string }) => client.put<User>('/api/v1/auth/me', data),
  changePassword: (data: { current_password: string; new_password: string }) => client.put('/api/v1/auth/me/password', data),
  forgotPassword: (email: string) => client.post('/api/v1/auth/forgot-password', { email }),
}
