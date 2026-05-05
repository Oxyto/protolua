module.exports = grammar({
  name: "protolua",

  extras: ($) => [/\s/, $.comment],

  word: ($) => $.identifier,

  rules: {
    source_file: ($) => repeat($._statement),

    _statement: ($) =>
      choice(
        $.local_statement,
        $.function_statement,
        $.event_statement,
        $.if_statement,
        $.while_statement,
        $.for_statement,
        $.return_statement,
        $.output_statement,
        $.write_statement,
        $.drive_statement,
        $.assignment_statement,
        $.call_statement
      ),

    local_statement: ($) => seq("local", $.identifier, optional($.type_annotation), optional(seq("=", $._expression))),
    function_statement: ($) => seq("function", $.identifier, $.parameters, optional($.outputs), repeat($._statement), "end"),
    event_statement: ($) => seq("on", choice($.identifier, $.string), optional($.parameters), optional($.outputs), "do", repeat($._statement), "end"),
    if_statement: ($) => seq("if", $._expression, "then", repeat($._statement), repeat(seq("elseif", $._expression, "then", repeat($._statement))), optional(seq("else", repeat($._statement))), "end"),
    while_statement: ($) => seq("while", $._expression, "do", repeat($._statement), "end"),
    for_statement: ($) => seq("for", $.identifier, "=", $._expression, ",", $._expression, optional(seq(",", $._expression)), "do", repeat($._statement), "end"),
    return_statement: ($) => seq("return", optional(seq($._expression, repeat(seq(",", $._expression))))),
    output_statement: ($) => seq("output", $.identifier, "=", $._expression),
    write_statement: ($) => seq("write", $._expression, "=", $._expression),
    drive_statement: ($) => seq("drive", $._expression, "=", $._expression),
    assignment_statement: ($) => seq($._expression, "=", $._expression),
    call_statement: ($) => $.call,

    parameters: ($) => seq("(", optional(seq($.parameter, repeat(seq(",", $.parameter)))), ")"),
    parameter: ($) => seq($.identifier, optional($.type_annotation)),
    outputs: ($) => seq("->", choice($.type_identifier, seq("(", optional(seq($.parameter, repeat(seq(",", $.parameter)))), ")"))),
    type_annotation: ($) => seq(":", $.type_identifier),

    _expression: ($) =>
      choice(
        $.identifier,
        $.pf_identifier,
        $.type_identifier,
        $.number,
        $.string,
        $.boolean,
        $.nil,
        $.table,
        $.call,
        $.member,
        $.binary_expression,
        $.unary_expression,
        seq("(", $._expression, ")")
      ),

    call: ($) => seq(choice($.identifier, $.pf_identifier, $.member), "(", optional(seq($._expression, repeat(seq(",", $._expression)))), ")"),
    member: ($) => seq(choice($.identifier, $.pf_identifier, $.call), ".", $.identifier),
    table: ($) => seq("{", optional(seq($.table_field, repeat(seq(choice(",", ";"), $.table_field)), optional(choice(",", ";")))), "}"),
    table_field: ($) => choice(seq($.identifier, "=", $._expression), $._expression),
    unary_expression: ($) => prec(6, seq(choice("-", "not", "#"), $._expression)),
    binary_expression: ($) => choice(
      prec.left(1, seq($._expression, choice("or", "and"), $._expression)),
      prec.left(2, seq($._expression, choice("==", "~=", "<", "<=", ">", ">="), $._expression)),
      prec.left(3, seq($._expression, choice("+", "-", ".."), $._expression)),
      prec.left(4, seq($._expression, choice("*", "/", "%"), $._expression)),
      prec.right(5, seq($._expression, "^", $._expression))
    ),

    boolean: (_) => choice("true", "false"),
    nil: (_) => "nil",
    pf_identifier: (_) => /pf(\.[A-Za-z_][A-Za-z0-9_]*)+/,
    type_identifier: (_) => /[A-Z][A-Za-z0-9_]*(\.[A-Z][A-Za-z0-9_]*)*|bool|int|float|double|string|float[234]|colorX?|quat|rect/,
    identifier: (_) => /[A-Za-z_][A-Za-z0-9_]*/,
    number: (_) => /\d+(\.\d+)?/,
    string: (_) => choice(seq('"', repeat(choice(/[^"\\]/, /\\./)), '"'), seq("'", repeat(choice(/[^'\\]/, /\\./)), "'")),
    comment: (_) => /--[^\n]*/,
  },
});
