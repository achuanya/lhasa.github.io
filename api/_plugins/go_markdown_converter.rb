module Jekyll
  class GoMarkdownConverter < Converter
    safe true
    priority :low

    def matches(ext)
      ext =~ /^\.md$/i
    end

    def output_ext(ext)
      ".html"
    end

    def convert(content)
      `echo #{Shellwords.escape(content)} | ./go_markdown_parser`
    end
  end
end
