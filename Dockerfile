FROM ruby:3.2

WORKDIR /app

COPY . /app

RUN bundle install

EXPOSE 8888

CMD ["bundle", "exec", "jekyll", "serve", "--host", "0.0.0.0", "--port", "8888"]