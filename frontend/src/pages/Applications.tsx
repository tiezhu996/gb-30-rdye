import { Button, Card, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  listMyApplications,
  listOrgApplications,
  updateApplicationStatus,
  withdrawApplication,
} from '@/api/application'
import ApplicationStatusBadge from '@/components/common/ApplicationStatusBadge'
import {
  ApplicationStatusMap,
  isActiveApplicationStatus,
  type ApplicationStatus,
} from '@/constants/application'
import { useAuth } from '@/hooks/useAuth'
import type { AdoptionApplication } from '@/types/api'
import { formatDate } from '@/utils/dateFormat'

// 每个进行中状态下，机构可一键推进到的下一环节。
const NEXT_PIPELINE_STATUS: Record<string, { value: ApplicationStatus; label: string }> = {
  submitted: { value: 'org_review', label: '受理并进入审核' },
  org_review: { value: 'communicating', label: '进入沟通' },
  communicating: { value: 'confirmed', label: '确认意向' },
  confirmed: { value: 'offline_interview', label: '安排线下面签' },
  offline_interview: { value: 'approved', label: '通过申请' },
}

// 申请问卷的三个固定问题（提交时由 ApplicationForm 写入）。
const QUESTIONNAIRE_FIELDS: { key: string; label: string }[] = [
  { key: 'family', label: '家庭情况' },
  { key: 'has_yard', label: '是否有庭院/阳台' },
  { key: 'pet_experience', label: '养宠经验' },
]

function QuestionnaireContent({ raw }: { raw: string }) {
  let data: Record<string, unknown> = {}
  try {
    data = JSON.parse(raw || '{}')
  } catch {
    data = {}
  }
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      {QUESTIONNAIRE_FIELDS.map((f) => (
        <div key={f.key}>
          <Typography.Text strong>{f.label}：</Typography.Text>
          <Typography.Text>
            {f.key === 'has_yard' ? (data[f.key] ? '是' : '否') : String(data[f.key] ?? '-')}
          </Typography.Text>
        </div>
      ))}
    </Space>
  )
}

export default function Applications() {
  const { isOrg } = useAuth()
  const [apps, setApps] = useState<AdoptionApplication[]>([])
  const [status, setStatus] = useState('')
  const [rejecting, setRejecting] = useState<AdoptionApplication | null>(null)
  const [rejectReason, setRejectReason] = useState('')
  const [withdrawing, setWithdrawing] = useState<AdoptionApplication | null>(null)
  const [withdrawReason, setWithdrawReason] = useState('')
  const [detail, setDetail] = useState<AdoptionApplication | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function load(s = status) {
    if (isOrg) {
      setApps(await listOrgApplications(s))
    } else {
      setApps(await listMyApplications())
    }
  }
  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOrg])

  // 机构推进审核流程（非拒绝的正向流转）。
  async function advance(r: AdoptionApplication) {
    const next = NEXT_PIPELINE_STATUS[r.status]
    if (!next) return
    await updateApplicationStatus(r.id, next.value)
    message.success(next.value === 'approved' ? '申请已通过，动物已记为已领养' : '状态已更新')
    await load()
  }

  async function confirmReject() {
    if (!rejectReason.trim()) {
      message.warning('请填写拒绝原因')
      return
    }
    setSubmitting(true)
    try {
      await updateApplicationStatus(rejecting!.id, 'rejected', rejectReason.trim())
      message.success('已拒绝申请，动物重新开放领养')
      setRejecting(null)
      setRejectReason('')
      await load()
    } finally {
      setSubmitting(false)
    }
  }

  async function confirmWithdraw() {
    if (!withdrawReason.trim()) {
      message.warning('请填写撤回原因')
      return
    }
    setSubmitting(true)
    try {
      await withdrawApplication(withdrawing!.id, withdrawReason.trim())
      message.success('申请已撤回，动物重新开放领养')
      setWithdrawing(null)
      setWithdrawReason('')
      await load()
    } finally {
      setSubmitting(false)
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: '动物',
      dataIndex: 'pet_id',
      render: (v: number) => <Link to={`/pets/${v}`}># {v}</Link>,
    },
    { title: '申请时间', dataIndex: 'created_at', width: 160, render: (v: string) => formatDate(v) },
    { title: '状态', dataIndex: 'status', width: 120, render: (v: string) => <ApplicationStatusBadge status={v} /> },
    {
      title: '结束原因',
      dataIndex: 'close_reason',
      render: (v: string, r: AdoptionApplication) =>
        v ? (
          <Typography.Text type={r.status === 'rejected' ? 'danger' : 'secondary'} ellipsis={{ tooltip: v }}>
            {r.status === 'rejected' ? '机构拒绝：' : r.status === 'withdrawn' ? '申请人撤回：' : ''}
            {v}
          </Typography.Text>
        ) : (
          '-'
        ),
    },
    {
      title: '操作',
      width: 240,
      render: (_: unknown, r: AdoptionApplication) => {
        const active = isActiveApplicationStatus(r.status)
        const buttons = [
          <Button key="detail" size="small" onClick={() => setDetail(r)}>
            查看问卷
          </Button>,
        ]
        if (isOrg && active) {
          const next = NEXT_PIPELINE_STATUS[r.status]
          if (next) {
            buttons.push(
              <Button key="advance" size="small" type="primary" onClick={() => advance(r)}>
                {r.status === 'offline_interview' ? '通过申请' : '推进'}
              </Button>,
            )
          }
          buttons.push(
            <Button key="reject" size="small" danger onClick={() => { setRejecting(r); setRejectReason('') }}>
              拒绝
            </Button>,
          )
        }
        // 申请人只能在申请进行中撤回；已通过（已领养）等终态记录不可再改。
        if (!isOrg && active) {
          buttons.push(
            <Button key="withdraw" size="small" danger onClick={() => { setWithdrawing(r); setWithdrawReason('') }}>
              撤回申请
            </Button>,
          )
        }
        return <Space wrap>{buttons}</Space>
      },
    },
  ]

  return (
    <div>
      <h1>{isOrg ? '收到的领养申请' : '我的领养申请'}</h1>
      {isOrg && (
        <Select
          style={{ width: 200, marginBottom: 12 }}
          placeholder="按状态筛选"
          allowClear
          value={status || undefined}
          onChange={(v) => {
            setStatus(v || '')
            load(v || '')
          }}
          options={(Object.keys(ApplicationStatusMap) as ApplicationStatus[]).map((s) => ({
            value: s,
            label: ApplicationStatusMap[s].text,
          }))}
        />
      )}
      <Table rowKey="id" dataSource={apps} pagination={false} columns={columns} />
      {!apps.length && <Card style={{ marginTop: 12 }}>暂无申请记录</Card>}

      {/* 机构拒绝：必须填写原因，原因会永久保留在申请记录里 */}
      <Modal
        title={`拒绝申请 #${rejecting?.id ?? ''}`}
        open={!!rejecting}
        onOk={confirmReject}
        confirmLoading={submitting}
        okText="确认拒绝"
        okButtonProps={{ danger: true }}
        onCancel={() => setRejecting(null)}
      >
        <p>拒绝后该动物将立即重新开放领养，其他领养人可再次递交申请。请填写拒绝原因：</p>
        <Input.TextArea
          rows={3}
          maxLength={500}
          showCount
          value={rejectReason}
          onChange={(e) => setRejectReason(e.target.value)}
          placeholder="例如：居住条件不符合饲养要求 / 资料不完整 / 已有更合适的申请人"
        />
      </Modal>

      {/* 申请人撤回：同样需要原因 */}
      <Modal
        title={`撤回申请 #${withdrawing?.id ?? ''}`}
        open={!!withdrawing}
        onOk={confirmWithdraw}
        confirmLoading={submitting}
        okText="确认撤回"
        okButtonProps={{ danger: true }}
        onCancel={() => setWithdrawing(null)}
      >
        <p>撤回后该动物将立即重新开放领养。请填写撤回原因：</p>
        <Input.TextArea
          rows={3}
          maxLength={500}
          showCount
          value={withdrawReason}
          onChange={(e) => setWithdrawReason(e.target.value)}
          placeholder="例如：家庭计划有变，暂时无法领养"
        />
      </Modal>

      {/* 申请详情：问卷 + 结束原因 */}
      <Modal title={`申请详情 #${detail?.id ?? ''}`} open={!!detail} footer={null} onCancel={() => setDetail(null)}>
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <ApplicationStatusBadge status={detail.status} />
              <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                申请时间：{formatDate(detail.created_at)}
              </Typography.Text>
            </div>
            <div>
              <Typography.Text strong>领养问卷</Typography.Text>
              <Card size="small" style={{ marginTop: 8 }}>
                <QuestionnaireContent raw={detail.questionnaire} />
              </Card>
            </div>
            {detail.close_reason && (
              <div>
                <Typography.Text strong>
                  {detail.status === 'rejected' ? '拒绝原因' : detail.status === 'withdrawn' ? '撤回原因' : '结束原因'}
                </Typography.Text>
                <Card size="small" style={{ marginTop: 8 }}>
                  <Tag color={detail.status === 'rejected' ? 'red' : 'default'}>
                    {ApplicationStatusMap[detail.status as ApplicationStatus]?.text}
                  </Tag>
                  {detail.close_reason}
                </Card>
              </div>
            )}
          </Space>
        )}
      </Modal>
    </div>
  )
}
