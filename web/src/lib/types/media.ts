export interface Property {
       name: string;
       type: string;
       value: string;
}

export interface MediaItem {
       id: string;
       url: string;
       width: number;
       height: number;
       format: string;
       properties: Property[];
}

export interface MediaDetail extends MediaItem {
	size: number;
	tags: string[];
}
