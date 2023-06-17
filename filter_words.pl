#!/usr/bin/perl

use strict;
use warnings;

print "edit_words := []string{\n";

while (<>) {
    chomp;
    if (length($_) < 5 || length($_) > 8) {
        next;
    }
    if (!/^[A-Za-z]+$/) {
        next;
    }
    $_ =~ tr/A-Z/a-z/;

    print "\t\"$_\", \n";
}

print "}\n";

