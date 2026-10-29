FROM node:22-alpine AS builder

WORKDIR /app

ARG NEXT_PUBLIC_API_BASE_URL=
ARG BACKEND_PROXY_URL=http://localhost:8081
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
ENV BACKEND_PROXY_URL=$BACKEND_PROXY_URL

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM node:22-alpine

RUN apk add --no-cache wget curl \
  && addgroup -g 10001 -S nodejs \
  && adduser -S -u 10001 -G nodejs nextjs

WORKDIR /app

ARG BACKEND_PROXY_URL=http://backend-api:8081
ENV BACKEND_PROXY_URL=$BACKEND_PROXY_URL
ENV NODE_ENV=production

COPY package*.json ./
RUN npm ci --omit=dev

COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/next.config.ts ./next.config.ts

RUN chown -R nextjs:nodejs /app

USER nextjs
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3000/health >/dev/null || exit 1
CMD ["npm", "run", "start"]
