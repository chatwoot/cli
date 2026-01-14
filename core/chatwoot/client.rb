# frozen_string_literal: true

require "faraday"
require "json"

module Chatwoot
  class Client
    attr_reader :account_id

    def initialize(base_url:, api_key:, account_id:)
      @base_url = base_url.chomp("/")
      @api_key = api_key
      @account_id = account_id
    end

    def conversations
      @conversations ||= Resources::Conversations.new(self)
    end

    def messages(conversation_id)
      Resources::Messages.new(self, conversation_id)
    end

    def labels(conversation_id)
      Resources::Labels.new(self, conversation_id)
    end

    def get(path, params = {})
      request(:get, path, params)
    end

    def post(path, body = {})
      request(:post, path, body)
    end

    def patch(path, body = {})
      request(:patch, path, body)
    end

    def delete(path)
      request(:delete, path)
    end

    private

    def connection
      @connection ||= Faraday.new(url: @base_url) do |f|
        f.request :json
        f.response :json, content_type: /\bjson$/
        f.headers["api_access_token"] = @api_key
        f.headers["Content-Type"] = "application/json"
      end
    end

    def request(method, path, payload = nil)
      full_path = "/api/v1/accounts/#{@account_id}#{path}"

      response = case method
                 when :get
                   connection.get(full_path, payload)
                 when :post
                   connection.post(full_path, payload)
                 when :patch
                   connection.patch(full_path, payload)
                 when :delete
                   connection.delete(full_path)
                 end

      handle_response(response)
    end

    def handle_response(response)
      case response.status
      when 200..299
        response.body
      when 401
        raise AuthenticationError, "Invalid API key"
      when 404
        raise NotFoundError, "Resource not found"
      when 429
        raise RateLimitError, "Rate limit exceeded"
      else
        raise APIError.new(
          "API error: #{response.status}",
          status: response.status,
          body: response.body
        )
      end
    end
  end
end
