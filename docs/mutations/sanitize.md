# Mutations: sanitize

The demonstrations of the controls whose patches sit in `sanitize/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A body that cannot be converted is refused with no Markdown

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** a panic in the conversion is no longer recovered
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestGuardedFailsClosed`, `TestGuardedFailsClosed/a_panic_carrying_body_text`
- **Break (2):** a recovered panic's value, which can carry body text, is written into the error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestGuardedFailsClosed`, `TestGuardedFailsClosed/a_panic_carrying_body_text`
- **Break (3):** a parse failure returns empty Markdown and no error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestFailClosed`, `TestFailClosed/600_nested_elements`, `TestFailClosed/600_nested_tables`
- **Break (4):** a failed conversion returns whatever output it produced with its error
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestGuardedFailsClosed`, `TestGuardedFailsClosed/an_error_with_partial_output`
- **Break (5):** a body exactly at the size limit is refused as over it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestABodyAtTheSizeLimitConverts`
- **Break (6):** a body over the size limit is converted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestFailClosed`, `TestFailClosed/a_body_one_byte_over_the_size_limit`

## A link keeps its target only when the target is an absolute http or https URL with a host, or a mailto URL of bare addresses and nothing else

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** an http or https target without a host is kept
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLinkTargets`, `TestLinkTargets/an_http_target_with_no_host`
- **Break (2):** a mailto target without a query is kept whatever its recipients are, none included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLinkTargets`, `TestLinkTargets/mailto_with_a_display_name`, `TestLinkTargets/mailto_with_an_empty_recipient`, `TestLinkTargets/mailto_with_no_domain`
- **Break (3):** the allow list loses mailto, so a mailto link is reduced to its label
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLinkTargets`, `TestLinkTargets/mailto`, `TestLinkTargets/mailto_in_capitals_with_surrounding_space`, `TestLinkTargets/mailto_with_two_recipients`, `TestTheConvertedBodysForm`
- **Break (4):** a mailto target keeps its query, so it can add recipients or fill in a message
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLinkTargets`, `TestLinkTargets/mailto_with_a_blind_copy_recipient`, `TestLinkTargets/mailto_with_a_copy_recipient_and_a_body`, `TestLinkTargets/mailto_with_a_fragment`, `TestLinkTargets/mailto_with_a_subject`, `TestLinkTargets/mailto_with_an_empty_query`, `TestTheConvertedBodysForm`
- **Break (5):** link targets are judged against a deny list of javascript: instead of an allow list
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLinkTargets`, `TestLinkTargets/a_local_file`, `TestLinkTargets/a_relative_path`, `TestLinkTargets/a_target_with_no_scheme`, `TestLinkTargets/an_HTML_data:_target`, `TestLinkTargets/an_http_target_with_no_host`, `TestLinkTargets/mailto_with_a_blind_copy_recipient`, `TestLinkTargets/mailto_with_a_copy_recipient_and_a_body`, `TestLinkTargets/mailto_with_a_display_name`, `TestLinkTargets/mailto_with_a_fragment`, `TestLinkTargets/mailto_with_a_subject`, `TestLinkTargets/mailto_with_an_empty_query`, `TestLinkTargets/mailto_with_an_empty_recipient`, `TestLinkTargets/mailto_with_no_domain`, `TestLinkTargets/mailto_with_no_recipient`, `TestLinkTargets/vbscript`, `TestTheConvertedBodysForm`
- **Break (6):** the pass emptying unsafe link targets is never registered
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLinkTargets`, `TestLinkTargets/a_local_file`, `TestLinkTargets/a_relative_path`, `TestLinkTargets/a_target_with_no_scheme`, `TestLinkTargets/an_HTML_data:_target`, `TestLinkTargets/an_http_target_with_no_host`, `TestLinkTargets/javascript`, `TestLinkTargets/javascript_after_space`, `TestLinkTargets/javascript_in_mixed_case`, `TestLinkTargets/javascript_split_by_a_tab`, `TestLinkTargets/javascript_split_by_an_encoded_newline`, `TestLinkTargets/mailto_with_a_blind_copy_recipient`, `TestLinkTargets/mailto_with_a_copy_recipient_and_a_body`, `TestLinkTargets/mailto_with_a_display_name`, `TestLinkTargets/mailto_with_a_fragment`, `TestLinkTargets/mailto_with_a_subject`, `TestLinkTargets/mailto_with_an_empty_query`, `TestLinkTargets/mailto_with_an_empty_recipient`, `TestLinkTargets/mailto_with_no_domain`, `TestLinkTargets/mailto_with_no_recipient`, `TestLinkTargets/vbscript`, `TestTheConvertedBodysForm`

## A released body carries no markup

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** the pass writing code as escaped text is never registered
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestNoMarkupInTheOutput`, `TestNoMarkupInTheOutput/a_code_block_in_a_blockquote`, `TestNoMarkupInTheOutput/inline_code`, `TestNoMarkupInTheOutput/preformatted_lines`, `TestNoMarkupInTheOutput/preformatted_markup_in_a_blockquote`, `TestNoMarkupInTheOutput/preformatted_markup_in_a_blockquote_in_a_list`, `TestNoMarkupInTheOutput/the_library's_own_marker_characters_in_text`, `TestTheConvertedBodysForm`
- **Break (2):** a preformatted block's line breaks are written as the text of a br tag instead of as br elements
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestNoMarkupInTheOutput`, `TestNoMarkupInTheOutput/a_code_block_in_a_blockquote`, `TestNoMarkupInTheOutput/preformatted_lines`, `TestNoMarkupInTheOutput/preformatted_markup_in_a_blockquote`, `TestNoMarkupInTheOutput/preformatted_markup_in_a_blockquote_in_a_list`
- **Break (3):** the HTML comment the library puts between adjacent lists is switched back on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestNoMarkupInTheOutput`, `TestNoMarkupInTheOutput/two_adjacent_lists`, `TestTheConvertedBodysForm`
- **Break (4):** text keeps the library's marker characters, so it can pose as one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestNoMarkupInTheOutput`, `TestNoMarkupInTheOutput/the_library's_own_marker_characters_in_text`
- **Break (5):** the pass flattens inline code but leaves preformatted text to the library's fenced blocks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestNoMarkupInTheOutput`, `TestNoMarkupInTheOutput/a_code_block_in_a_blockquote`, `TestNoMarkupInTheOutput/preformatted_lines`, `TestNoMarkupInTheOutput/preformatted_markup_in_a_blockquote`, `TestNoMarkupInTheOutput/preformatted_markup_in_a_blockquote_in_a_list`, `TestTheConvertedBodysForm`

## Elements that execute, embed, style or collect input are dropped with their content

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** every element on the list is kept as a block with its content instead of dropped
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestActiveContentIsDropped`, `TestActiveContentIsDropped/a_canvas_and_its_fallback`, `TestActiveContentIsDropped/a_form_and_its_controls`, `TestActiveContentIsDropped/a_math_element`, `TestActiveContentIsDropped/a_noscript_fallback`, `TestActiveContentIsDropped/a_script`, `TestActiveContentIsDropped/a_select_and_its_options`, `TestActiveContentIsDropped/a_style`, `TestActiveContentIsDropped/a_template`, `TestActiveContentIsDropped/a_text_area`, `TestActiveContentIsDropped/a_title_in_the_head`, `TestActiveContentIsDropped/a_video_and_its_fallback`, `TestActiveContentIsDropped/an_audio_and_its_fallback`, `TestActiveContentIsDropped/an_iframe`, `TestActiveContentIsDropped/an_object_and_its_fallback`, `TestActiveContentIsDropped/an_svg_and_its_text`, `TestImagesAreDropped`, `TestImagesAreDropped/a_link_holding_only_an_image`, `TestImagesAreDropped/a_picture_with_sources`, `TestImagesAreDropped/a_remote_image`, `TestImagesAreDropped/an_embedded_cid:_image`, `TestImagesAreDropped/an_inline_data:_image`, `TestRawHTMLALinkAndARemoteImage`, `TestTheConvertedBodysForm`
- **Break (2):** no element on the list is dropped, leaving only the library's defaults
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestActiveContentIsDropped`, `TestActiveContentIsDropped/a_canvas_and_its_fallback`, `TestActiveContentIsDropped/a_form_and_its_controls`, `TestActiveContentIsDropped/a_math_element`, `TestActiveContentIsDropped/a_select_and_its_options`, `TestActiveContentIsDropped/a_template`, `TestActiveContentIsDropped/a_title_in_the_head`, `TestActiveContentIsDropped/a_video_and_its_fallback`, `TestActiveContentIsDropped/an_audio_and_its_fallback`, `TestActiveContentIsDropped/an_object_and_its_fallback`, `TestActiveContentIsDropped/an_svg_and_its_text`, `TestImagesAreDropped`, `TestImagesAreDropped/a_link_holding_only_an_image`, `TestImagesAreDropped/a_picture_with_sources`, `TestImagesAreDropped/a_remote_image`, `TestImagesAreDropped/an_embedded_cid:_image`, `TestImagesAreDropped/an_inline_data:_image`, `TestRawHTMLALinkAndARemoteImage`, `TestTheConvertedBodysForm`

## Every image is dropped from a released body

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** images are no longer on the list of dropped elements
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestImagesAreDropped`, `TestImagesAreDropped/a_link_holding_only_an_image`, `TestImagesAreDropped/a_remote_image`, `TestImagesAreDropped/an_embedded_cid:_image`, `TestImagesAreDropped/an_inline_data:_image`, `TestRawHTMLALinkAndARemoteImage`, `TestTheConvertedBodysForm`
- **Break (2):** an image is dropped but its alternative text is left in its place
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestImagesAreDropped`, `TestImagesAreDropped/a_link_holding_only_an_image`, `TestImagesAreDropped/a_remote_image`, `TestImagesAreDropped/an_embedded_cid:_image`, `TestImagesAreDropped/an_inline_data:_image`, `TestRawHTMLALinkAndARemoteImage`, `TestTheConvertedBodysForm`

## Message text with no HTML form is released as one fenced code block that shows it exactly

- **Date · evidence:** 2026-10-01 · [pull request #235](https://github.com/ppat/mediated-mailbox-mcp/pull/235)
- **Break (1):** the fence is as long as the longest run of backticks in the text rather than one longer, so that run closes the block
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLiteralForms`, `TestLiteralTextIsOneCodeBlockShowingIt`
- **Break (2):** the fence is always three backticks, so a text holding a run of three closes the block early
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLiteralForms`, `TestLiteralTextIsOneCodeBlockShowingIt`
- **Break (3):** the text is released as it is, with no fence around it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestLiteralForms`, `TestLiteralTextIsOneCodeBlockShowingIt`

## The conversion writes the structure the Content Scanner reads

- **Date · evidence:** 2026-09-24 · [pull request #152](https://github.com/ppat/mediated-mailbox-mcp/pull/152)
- **Break (1):** the cells of a table the table plugin refuses run together on one line
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestStructureTheScannerReads`, `TestStructureTheScannerReads/a_layout_table`
- **Break (2):** headings are written underlined instead of as lines starting with #
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestRawHTMLALinkAndARemoteImage`, `TestStructureTheScannerReads`, `TestStructureTheScannerReads/headings`
- **Break (3):** bold is written with underscores instead of asterisks
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/sanitize/markdown`:** `TestRawHTMLALinkAndARemoteImage`, `TestStructureTheScannerReads`, `TestStructureTheScannerReads/bold`
