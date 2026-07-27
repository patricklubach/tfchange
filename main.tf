resource "local_file" "foo" {
  content  = "foo!lol"
  filename = "${path.module}/foo.txt"
}
