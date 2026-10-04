import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from '@testing-library/react'
import React, { createContext } from 'react'
import { QueryClient } from '@tanstack/react-query'
import SocketProvider from './SocketProvider'
import { RealtimeManager } from './RealtimeManager'
import { useChannel, useChannelEvent } from './context/socket'
import { useAppStore } from '@/stores'
import { installFakeSocket, freezeJitter, type SocketEnv } from './socketTestHarness'
import { createPassthroughRouter, renderRouter } from '@/test/routerHarness'

vi.mock('@/lib/api/client/app/socket/getSocket', () => ({
    default: vi.fn(async () => ({ url: 'ws://localhost:4000/socket/websocket?token=test' })),
}))

const USER_ID = '99999999-9999-9999-9999-999999999999'
const ORG_A = '11111111-1111-1111-1111-111111111111'
const ORG_B = '22222222-2222-2222-2222-222222222222'

vi.mock('./context/user', () => ({
    UserContext: createContext(null),
    useUserProfile: () => ({ user: { id: USER_ID } }),
}))

vi.mock('@/lib/api/hooks/app/unibox/useUnseenCount', () => ({
    default: () => ({ data: undefined }),
}))

let env: SocketEnv
let queryClient: QueryClient

const CAMPAIGN = 'campaign:33333333-3333-3333-3333-333333333333'

/** A page-level surface holding its own topic, the way useCampaignChannel does. */
function CampaignPanel({ onEvent }: { onEvent: () => void }) {
    useChannel(CAMPAIGN)
    useChannelEvent(CAMPAIGN, 'EMAIL_SENT', () => onEvent())
    return null
}

/** A surface that only listens on the org channel, without holding the join. */
function OrgListener({ orgId, onEvent }: { orgId: string; onEvent: () => void }) {
    useChannelEvent(`org:${orgId}`, 'EMAIL_SENT', () => onEvent())
    return null
}

function Tree({ children }: { children?: React.ReactNode }) {
    return (
        <SocketProvider>
            <RealtimeManager>{children}</RealtimeManager>
        </SocketProvider>
    )
}

async function mount(children?: React.ReactNode) {
    const result = await renderRouter(createPassthroughRouter(<Tree>{children}</Tree>), queryClient)
    await act(async () => {
        await vi.advanceTimersByTimeAsync(10)
    })
    return result
}

async function tick(ms: number) {
    await act(async () => {
        await vi.advanceTimersByTimeAsync(ms)
    })
}

/** Ack every join the client has sent that the server hasn't answered yet. */
let ackedJoins = 0
async function ackPendingJoins() {
    const joins = env.joins()
    await act(async () => {
        for (let i = ackedJoins; i < joins.length; i++) env.ackJoin(joins[i])
    })
    ackedJoins = joins.length
}

/** Kill the live socket and let the provider's backoff bring a new one up. */
async function dropAndReconnect() {
    await act(async () => {
        env.instances[env.instances.length - 1].close()
    })
    await tick(500)
    await ackPendingJoins()
}

beforeEach(() => {
    vi.useFakeTimers()
    freezeJitter(0)
    env = installFakeSocket()
    ackedJoins = 0
    queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
    })
    useAppStore.setState({ currentOrganization: { id: ORG_A } as never, joinedChannels: [] })
})

afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
})

describe('RealtimeManager channel lifecycle', () => {
    it('joins the user and org channels exactly once on connect', async () => {
        await mount()
        await ackPendingJoins()

        expect(env.joins(`user:${USER_ID}`)).toHaveLength(1)
        expect(env.joins(`org:${ORG_A}`)).toHaveLength(1)
        expect(env.leaves()).toHaveLength(0)
        expect(useAppStore.getState().joinedChannels).toContain(`org:${ORG_A}`)
    })

    it('rejoins the org channel once per reconnect and keeps its subscribers', async () => {
        // The cleanup leaves while the socket is down; that must not strand the
        // rejoin or take other surfaces' handlers with it.
        const onEvent = vi.fn()
        await mount(<OrgListener orgId={ORG_A} onEvent={onEvent} />)
        await ackPendingJoins()

        await dropAndReconnect()

        expect(env.joins(`org:${ORG_A}`)).toHaveLength(2)
        expect(env.joins(`user:${USER_ID}`)).toHaveLength(2)

        await act(async () => {
            env.pushEvent(`org:${ORG_A}`, 'EMAIL_SENT', {})
        })
        expect(onEvent).toHaveBeenCalledTimes(1)
    })

    it('still routes org events into the query cache after a reconnect', async () => {
        await mount()
        await ackPendingJoins()
        await dropAndReconnect()

        const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
        await act(async () => {
            env.pushEvent(`org:${ORG_A}`, 'AUDIT_CREATED', {
                action: 'contact.updated',
                entity_type: 'contact',
                entity_id: 'abc',
            })
        })

        expect(
            invalidate.mock.calls.some(
                (c) => JSON.stringify((c[0] as { queryKey?: unknown })?.queryKey) === '["contacts"]'
            )
        ).toBe(true)
    })

    it('really leaves the previous workspace after any number of reconnects', async () => {
        // An unbalanced join per reconnect would pin the old org's channel open
        // and keep feeding the user another workspace's events.
        await mount()
        await ackPendingJoins()

        for (let i = 0; i < 3; i++) await dropAndReconnect()

        await act(async () => {
            useAppStore.setState({ currentOrganization: { id: ORG_B } as never })
        })
        await ackPendingJoins()

        expect(env.leaves(`org:${ORG_A}`)).toHaveLength(1)
        expect(env.joins(`org:${ORG_B}`)).toHaveLength(1)
        expect(useAppStore.getState().joinedChannels).not.toContain(`org:${ORG_A}`)
        expect(useAppStore.getState().joinedChannels).toContain(`org:${ORG_B}`)
    })

    it('joins the org channel once when the reconnect batches with the drop', async () => {
        // Comes back via rejoinChannels or via the effect depending on whether
        // React flushed the disconnected render; either way, exactly one join.
        const onEvent = vi.fn()
        await mount(<OrgListener orgId={ORG_A} onEvent={onEvent} />)
        await ackPendingJoins()

        await act(async () => {
            env.instances[env.instances.length - 1].close()
            await vi.advanceTimersByTimeAsync(500)
        })
        await ackPendingJoins()

        expect(env.joins(`org:${ORG_A}`)).toHaveLength(2)
        await act(async () => {
            env.pushEvent(`org:${ORG_A}`, 'EMAIL_SENT', {})
        })
        expect(onEvent).toHaveBeenCalledTimes(1)
    })

    it('does not rejoin a workspace abandoned while the socket was down', async () => {
        await mount()
        await ackPendingJoins()

        await act(async () => {
            env.instances[env.instances.length - 1].close()
        })

        // The user picks another workspace before the socket is back.
        await act(async () => {
            useAppStore.setState({ currentOrganization: { id: ORG_B } as never })
        })
        await tick(500)
        await ackPendingJoins()

        expect(env.joins(`org:${ORG_A}`)).toHaveLength(1)
        expect(env.joins(`org:${ORG_B}`)).toHaveLength(1)
        expect(env.joins(`user:${USER_ID}`)).toHaveLength(2)
    })

    it('rejoins a page-owned channel alongside the org channel', async () => {
        // Effect-driven rejoin and rejoinChannels both run on the same reconnect.
        const onEvent = vi.fn()
        await mount(<CampaignPanel onEvent={onEvent} />)
        await ackPendingJoins()

        await dropAndReconnect()

        expect(env.joins(CAMPAIGN)).toHaveLength(2)
        expect(env.joins(`org:${ORG_A}`)).toHaveLength(2)
        expect(env.leaves(CAMPAIGN)).toHaveLength(0)

        await act(async () => {
            env.pushEvent(CAMPAIGN, 'EMAIL_SENT', {})
        })
        expect(onEvent).toHaveBeenCalledTimes(1)
    })

    it('stops delivering the previous workspace events after a switch', async () => {
        const onEvent = vi.fn()
        await mount(<OrgListener orgId={ORG_A} onEvent={onEvent} />)
        await ackPendingJoins()

        await act(async () => {
            useAppStore.setState({ currentOrganization: { id: ORG_B } as never })
        })
        await ackPendingJoins()

        // The server stops sending on a left topic; assert the client asked it to.
        expect(env.leaves(`org:${ORG_A}`)).toHaveLength(1)
        expect(env.joins(`org:${ORG_A}`)).toHaveLength(1)

        // And a later reconnect must not resurrect the abandoned topic.
        await dropAndReconnect()
        expect(env.joins(`org:${ORG_A}`)).toHaveLength(1)
        expect(env.joins(`org:${ORG_B}`)).toHaveLength(2)
        expect(onEvent).not.toHaveBeenCalled()
    })
})
