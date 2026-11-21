export function run(note) {
  const tasks = [];
  let fence = "";
  for (const line of note.content.split(/\r?\n/)) {
    const delimiter = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(line);
    if (delimiter) {
      if (!fence) fence = delimiter[1];
      else if (delimiter[1][0] === fence[0] && delimiter[1].length >= fence.length && !delimiter[2].trim()) fence = "";
      continue;
    }
    if (!fence && /^\s*(?:[-*+]|\d+[.)])\s+\[ \]\s+\S/.test(line)) tasks.push(line.trim());
  }
  return tasks.join("\n");
}
