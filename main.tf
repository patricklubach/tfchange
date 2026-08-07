# resource "local_file" "foo" {
#   content  = "foo!lol"
#   filename = "${path.module}/foo.txt"
# }

# resource "local_file" "bar" {
#   content  = "bar!lol"
#   filename = "${path.module}/bar.txt"
# }

resource "local_file" "baz" {
  content  = "foo!lol"
  filename = "${path.module}/bazz.txt"
}

resource "local_file" "lel" {
  content  = "baz!lel"
  filename = "${path.module}/lel.txt"
}

resource "local_file" "lullal" {
  content  = "baz!lel"
  filename = "${path.module}/lul.txt"
}
