FROM ruby:3.2.2-slim-bullseye AS base

ENV LANG=C.UTF-8 \
    BUNDLE_JOBS=4 \
    BUNDLE_RETRY=3 \
    GEM_HOME="/usr/local/bundle" \
    PATH="$GEM_HOME/bin:$PATH" \
    NVM_DIR="/root/.nvm"

RUN sed -i 's@http://deb.debian.org@http://mirrors.aliyun.com@g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    curl gnupg git build-essential && \
    rm -rf /var/lib/apt/lists/*

FROM base AS node
COPY install.sh /tmp/install.sh
RUN bash /tmp/install.sh && \
    . "$NVM_DIR/nvm.sh" && \
    nvm install 20.19.0 && \
    npm config set prefix "$NVM_DIR/versions/node/v20.19.0" && \
    npm install -g yarn && \
    rm /tmp/install.sh

ENV PATH="/root/.nvm/versions/node/v20.19.0/bin:$PATH"

FROM node AS deps
WORKDIR /app
COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

FROM deps AS builder
COPY . .
RUN yarn build

FROM base
COPY --from=node /root/.nvm /root/.nvm
ENV PATH="/root/.nvm/versions/node/v20.19.0/bin:/root/.nvm/versions/node/v20.19.0/lib/node_modules/.bin:$PATH" \
    NVM_DIR="/root/.nvm" \
    NODE_VERSION="20.19.0" \
    NODE_ENV="production"

WORKDIR /app
COPY --from=builder /app/assets ./assets
COPY --from=deps /app/node_modules ./node_modules
COPY Gemfile Gemfile.lock ./
RUN gem install bundler && bundle install --jobs=4 --retry=3 --without development test
COPY . .

COPY start-dev-server.sh /usr/local/bin/start-dev-server.sh
RUN chmod +x /usr/local/bin/start-dev-server.sh
RUN ls -l /usr/local/bin

ENTRYPOINT ["start-dev-server.sh"]

EXPOSE 8888