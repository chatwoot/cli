# frozen_string_literal: true

require "bubbletea"

module ChatwootTui
  class CLI
    def self.start(args = ARGV)
      new(args).run
    end

    def initialize(args)
      @args = args
    end

    def run
      case @args.first
      when "-v", "--version"
        puts "chatwoot-tui #{VERSION}"
      when "-h", "--help"
        show_help
      else
        start_app
      end
    end

    private

    def show_help
      puts <<~HELP
        chatwoot-tui - Terminal UI for Chatwoot

        Usage: chatwoot-tui [options]

        Options:
          -h, --help     Show this help
          -v, --version  Show version

        Environment Variables:
          CHATWOOT_BASE_URL   Your Chatwoot instance URL
          CHATWOOT_API_KEY    Your API access token
          CHATWOOT_ACCOUNT_ID Your account ID
      HELP
    end

    def start_app
      model = Model.new(
        base_url: ENV.fetch("CHATWOOT_BASE_URL"),
        api_key: ENV.fetch("CHATWOOT_API_KEY"),
        account_id: ENV.fetch("CHATWOOT_ACCOUNT_ID").to_i
      )
      Bubbletea.run(model, alt_screen: true)
    end
  end
end
