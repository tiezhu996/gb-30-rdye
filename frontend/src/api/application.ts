import request from '@/utils/request'
import type { AdoptionApplication } from '@/types/api'

export function submitApplication(payload: { pet_id: number; questionnaire?: string }) {
  return request.post<never, AdoptionApplication>('/applications', payload)
}

export function listMyApplications() {
  return request.get<never, AdoptionApplication[]>('/applications/me')
}

export function listOrgApplications(status?: string) {
  return request.get<never, AdoptionApplication[]>('/applications/org', { params: { status } })
}

// 申请状态流转：机构推进/拒绝（拒绝需带原因），申请人撤回（需带原因）。
export function updateApplicationStatus(id: number, status: string, reason?: string) {
  return request.put<never, AdoptionApplication>(`/applications/${id}/status`, { status, reason })
}

// 申请人撤回自己的进行中申请。
export function withdrawApplication(id: number, reason: string) {
  return updateApplicationStatus(id, 'withdrawn', reason)
}
