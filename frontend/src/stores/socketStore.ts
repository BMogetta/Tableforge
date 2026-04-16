import { create } from 'zustand'
import { GatewaySocket } from '@/lib/ws'

interface SocketState {
  gateway: GatewaySocket | null
  connectGateway: (url: string) => void
  disconnectGateway: () => void
}

export const useSocketStore = create<SocketState>((set, get) => ({
  gateway: null,

  connectGateway: (url: string) => {
    const existing = get().gateway
    if (existing && existing.url === url) return
    existing?.close()
    const gw = new GatewaySocket(url)
    gw.connect()
    set({ gateway: gw })
  },

  disconnectGateway: () => {
    get().gateway?.close()
    set({ gateway: null })
  },
}))
