export type ApplicationStatus =
  | 'submitted'
  | 'org_review'
  | 'communicating'
  | 'confirmed'
  | 'offline_interview'
  | 'approved'
  | 'rejected'
  | 'withdrawn'

export const ApplicationStatusMap: Record<ApplicationStatus, { text: string; color: string }> = {
  submitted: { text: '已提交', color: 'blue' },
  org_review: { text: '机构审核中', color: 'gold' },
  communicating: { text: '沟通中', color: 'cyan' },
  confirmed: { text: '已确认', color: 'geekblue' },
  offline_interview: { text: '线下面签', color: 'purple' },
  approved: { text: '已通过', color: 'green' },
  rejected: { text: '已拒绝', color: 'red' },
  withdrawn: { text: '已撤回', color: 'default' },
}

// 进行中的申请：宠物处于申请中(pending)，机构可操作、申请人可撤回。
export const ActiveApplicationStatuses: ApplicationStatus[] = [
  'submitted',
  'org_review',
  'communicating',
  'confirmed',
  'offline_interview',
]

// 终态：记录不可再改。
export const TerminalApplicationStatuses: ApplicationStatus[] = ['approved', 'rejected', 'withdrawn']

export function isActiveApplicationStatus(status: string): boolean {
  return ActiveApplicationStatuses.includes(status as ApplicationStatus)
}
