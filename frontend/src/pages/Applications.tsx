import { Button, Card, Input, Modal, Popconfirm, Select, Space, Table, Tag, message } from 'antd'
import { useEffect, useState } from 'react'
import {
  listMyApplications,
  listOrgApplications,
  rejectApplication,
  updateApplicationStatus,
  withdrawApplication,
} from '@/api/application'
import ApplicationStatusBadge from '@/components/common/ApplicationStatusBadge'
import { ApplicationStatusMap, OrgAdvanceMap, TerminalStatuses, type ApplicationStatus } from '@/constants/application'
import { useAuth } from '@/hooks/useAuth'
import type { AdoptionApplication } from '@/types/api'
import { formatDate } from '@/utils/dateFormat'

const ALL_STATUSES = Object.keys(ApplicationStatusMap) as ApplicationStatus[]

function isActive(status: string) {
  return !TerminalStatuses.includes(status as ApplicationStatus)
}

export default function Applications() {
  const { isOrg } = useAuth()
  const [apps, setApps] = useState<AdoptionApplication[]>([])
  const [status, setStatus] = useState('')
  const [rejecting, setRejecting] = useState<AdoptionApplication | null>(null)
  const [rejectReason, setRejectReason] = useState('')
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
  }, [isOrg])

  async function advance(r: AdoptionApplication) {
    const step = OrgAdvanceMap[r.status as ApplicationStatus]
    if (!step) return
    await updateApplicationStatus(r.id, step.next)
    message.success('申请已推进到下一环节')
    await load()
  }

  async function confirmReject() {
    if (!rejectReason.trim()) {
      message.warning('请填写拒绝原因')
      return
    }
    setSubmitting(true)
    try {
      await rejectApplication(rejecting!.id, rejectReason.trim())
      message.success('已拒绝申请，动物已回到待领养')
      setRejecting(null)
      setRejectReason('')
      await load()
    } finally {
      setSubmitting(false)
    }
  }

  async function withdraw(r: AdoptionApplication) {
    await withdrawApplication(r.id)
    message.success('申请已撤回，动物已回到待领养')
    await load()
  }

  return (
    <div>
      <h1>领养申请</h1>
      {isOrg && (
        <Select
          style={{ width: 180, marginBottom: 12 }}
          placeholder="按状态筛选"
          allowClear
          value={status || undefined}
          onChange={(v) => {
            setStatus(v || '')
            load(v || '')
          }}
          options={ALL_STATUSES.map((s) => ({ value: s, label: ApplicationStatusMap[s].text }))}
        />
      )}
      <Table
        rowKey="id"
        dataSource={apps}
        pagination={false}
        columns={[
          { title: 'ID', dataIndex: 'id' },
          { title: '宠物 ID', dataIndex: 'pet_id' },
          { title: '申请时间', dataIndex: 'created_at', render: (v: string) => formatDate(v) },
          { title: '状态', dataIndex: 'status', render: (v: string) => <ApplicationStatusBadge status={v} /> },
          {
            title: '结束原因',
            dataIndex: 'close_reason',
            render: (v: string, r) =>
              r.status === 'rejected' ? (
                <Tag color="red">{v || '机构审核未通过'}</Tag>
              ) : r.status === 'withdrawn' ? (
                <Tag>{v || '申请人主动撤回申请'}</Tag>
              ) : (
                '-'
              ),
          },
          {
            title: '操作',
            render: (_, r) => {
              if (!isActive(r.status)) {
                // approved / rejected / withdrawn are locked.
                return <span style={{ color: '#999' }}>已结束，不可修改</span>
              }
              if (isOrg) {
                const step = OrgAdvanceMap[r.status as ApplicationStatus]
                return (
                  <Space>
                    {step && (
                      <Button type="primary" size="small" onClick={() => advance(r)}>
                        {step.text}
                      </Button>
                    )}
                    <Button danger size="small" onClick={() => setRejecting(r)}>
                      拒绝
                    </Button>
                  </Space>
                )
              }
              return (
                <Popconfirm
                  title="撤回该申请？"
                  description="撤回后动物会重新开放领养，其他领养人可以申请。"
                  okText="确认撤回"
                  cancelText="再想想"
                  onConfirm={() => withdraw(r)}
                >
                  <Button size="small">撤回申请</Button>
                </Popconfirm>
              )
            },
          },
        ]}
      />
      {!apps.length && <Card style={{ marginTop: 12 }}>暂无申请记录</Card>}

      <Modal
        title={rejecting ? `拒绝申请 #${rejecting.id}` : ''}
        open={!!rejecting}
        confirmLoading={submitting}
        okText="确认拒绝"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        onOk={confirmReject}
        onCancel={() => {
          setRejecting(null)
          setRejectReason('')
        }}
      >
        <p style={{ color: '#999' }}>拒绝后该动物将立即回到待领养列表，其他领养人可再次申请。拒绝原因会保留在申请记录中。</p>
        <Input.TextArea
          rows={3}
          value={rejectReason}
          maxLength={500}
          showCount
          placeholder="请说明拒绝原因，例如：居住条件不符合、养宠经验不足等"
          onChange={(e) => setRejectReason(e.target.value)}
        />
      </Modal>
    </div>
  )
}
