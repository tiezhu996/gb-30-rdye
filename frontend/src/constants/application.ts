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

// Statuses in which the application is closed and can no longer change.
export const TerminalStatuses: ApplicationStatus[] = ['approved', 'rejected', 'withdrawn']

// Statuses the org can advance an application through.
export const OrgAdvanceMap: Partial<Record<ApplicationStatus, { next: ApplicationStatus; text: string }>> = {
  submitted: { next: 'org_review', text: '开始审核' },
  org_review: { next: 'communicating', text: '进入沟通' },
  communicating: { next: 'confirmed', text: '确认申请' },
  confirmed: { next: 'offline_interview', text: '安排面签' },
  offline_interview: { next: 'approved', text: '通过申请' },
}
