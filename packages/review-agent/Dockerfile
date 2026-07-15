# The project runs on tsx (tsconfig is noEmit + allowImportingTsExtensions, and
# imports are extensionless), so there is no `dist` to compile to — run the TS
# directly via tsx, installed from node_modules (npm ci includes it; npx uses
# the local binary, no network fetch at runtime).
FROM node:22-slim
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY tsconfig.json ./
COPY src/ ./src/
EXPOSE 4100
CMD ["npx", "tsx", "src/index.ts"]
