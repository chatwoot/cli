# frozen_string_literal: true

# Load charm-native first to ensure single Go runtime
require "charm_native"

require_relative "chatwoot_tui/version"
require_relative "chatwoot_tui/styles"
require_relative "chatwoot_tui/cli"
require_relative "chatwoot_tui/model"
require_relative "chatwoot_tui/components/conversations_list"

module ChatwootTui
  class Error < StandardError; end
end
