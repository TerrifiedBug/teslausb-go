type MessageHandler = (data: any) => void;

export class WSClient {
  private ws: WebSocket | null = null;
  private handlers: MessageHandler[] = [];
  private reconnectTimer: number | null = null;

  connect() {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    this.ws = new WebSocket(`${proto}//${window.location.host}/api/ws`);

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handlers.forEach(h => h(data));
      } catch {}
    };

    this.ws.onclose = () => {
      this.reconnectTimer = window.setTimeout(() => this.connect(), 3000);
    };
  }

  onMessage(handler: MessageHandler) {
    this.handlers.push(handler);
  }

  disconnect() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.ws?.close();
  }
}
