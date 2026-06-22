export interface DrawioConvertOptions {
  padding?: number;
  fontFamily?: string;
}

export function convert(drawioXml: string, options?: DrawioConvertOptions): string;
