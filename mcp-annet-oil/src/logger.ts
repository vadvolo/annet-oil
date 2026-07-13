import pino from 'pino';

const level = process.env.LOG_LEVEL || 'info';

export const logger = pino({
  level,
  transport: {
    target: 'pino/file',
    options: { destination: 2 }, // stderr
  },
  formatters: {
    level: (label) => ({ level: label }),
  },
  base: {
    service: 'mcp-annet-oil',
  },
});

export function createRequestLogger(requestId?: string) {
  return requestId ? logger.child({ request_id: requestId }) : logger;
}
