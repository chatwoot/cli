# frozen_string_literal: true

require_relative "chatwoot/client"
require_relative "chatwoot/resources/conversations"
require_relative "chatwoot/resources/messages"
require_relative "chatwoot/resources/labels"

module Chatwoot
  class Error < StandardError; end
  class AuthenticationError < Error; end
  class NotFoundError < Error; end
  class RateLimitError < Error; end
  class APIError < Error
    attr_reader :status, :body

    def initialize(message, status: nil, body: nil)
      @status = status
      @body = body
      super(message)
    end
  end
end
