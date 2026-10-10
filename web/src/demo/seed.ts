import type { Post } from '../api'

/** Demo account shown in the banner. Seeded posts by other users are read-only. */
export const DEMO_USER = 'rainy_day'
export const DEMO_PASSWORD = 'drizzle-demo'

const commons = (file: string) => `https://commons.wikimedia.org/wiki/File:${file}`
const thumb = (path: string) => `https://thumb.wikimedia.org/wikipedia/commons/thumb/${path}`

type Seed = Omit<Post, 'id' | 'createdAt' | 'mediaType'> & { hoursAgo: number }

// Handles and captions are made up; the photos are real, from Wikimedia Commons.
const SEEDS: Seed[] = [
  {
    username: 'fog_watcher',
    hoursAgo: 2,
    message: 'Space Needle playing hide and seek in the fog. Classic November in Seattle.',
    mediaUrl: thumb('3/30/Seattle_Space_Needle_in_the_fog_%2844897628634%29.jpg/960px-Seattle_Space_Needle_in_the_fog_%2844897628634%29.jpg'),
    credit: { author: 'Andy Reago & Chrissy McClarren', license: 'CC BY 2.0', url: commons('Seattle_Space_Needle_in_the_fog_(44897628634).jpg') },
  },
  {
    username: 'ferry_commuter',
    hoursAgo: 7,
    message: 'Ferry ride across Elliott Bay and the mountain decided to come out. Best commute in the city.',
    mediaUrl: thumb('2/24/MV_Suquamish%2C_Mount_Rainier%2C_and_the_Seattle_skyline.jpg/960px-MV_Suquamish%2C_Mount_Rainier%2C_and_the_Seattle_skyline.jpg'),
    credit: { author: 'SounderBruce', license: 'CC BY-SA 4.0', url: commons('MV_Suquamish,_Mount_Rainier,_and_the_Seattle_skyline.jpg') },
  },
  {
    username: 'suzzallo_studier',
    hoursAgo: 20,
    message: 'Suzzallo reading room on a rainy Tuesday. Silent study, very tall windows, three midterms to go.',
    mediaUrl: thumb('6/63/Suzzallo_Reading_Room_University_of_Washington_restoration_Seattle_Washington_2026.jpg/960px-Suzzallo_Reading_Room_University_of_Washington_restoration_Seattle_Washington_2026.jpg'),
    credit: { author: 'Guywelch2000', license: 'CC BY 4.0', url: commons('Suzzallo_Reading_Room_University_of_Washington_restoration_Seattle_Washington_2026.jpg') },
  },
  {
    username: 'gasworks_kites',
    hoursAgo: 31,
    message: 'Gas Works Park between showers. Rusty towers, a green hill, and kites everywhere once the rain stops.',
    mediaUrl: thumb('7/7d/Gas_Works_Park%2C_Seattle%2C_Washington%2C_USA.jpg/960px-Gas_Works_Park%2C_Seattle%2C_Washington%2C_USA.jpg'),
    credit: { author: 'Rick Gleason', license: 'CC BY-SA 4.0', url: commons('Gas_Works_Park,_Seattle,_Washington,_USA.jpg') },
  },
  {
    username: 'uw_quad_walks',
    hoursAgo: 52,
    message: 'Cherry blossom week on the Quad. Get there before 8 am if you want a photo without a thousand umbrellas.',
    mediaUrl: thumb('d/dd/University_of_Washington_Quad_cherry_blossoms_2017_-_02.jpg/960px-University_of_Washington_Quad_cherry_blossoms_2017_-_02.jpg'),
    credit: { author: 'Joe Mabel', license: 'CC BY-SA 4.0', url: commons('University_of_Washington_Quad_cherry_blossoms_2017_-_02.jpg') },
  },
  {
    username: 'pike_place_mornings',
    hoursAgo: 75,
    message: 'Pike Place before the crowds: coffee in hand, the big clock, and the market slowly waking up.',
    mediaUrl: thumb('d/de/Pike_Place_Market_sign_%28%22Public_Market_Center%22%29_with_clock%2C_Seattle%2C_Washington_LCCN2011630302.tif/lossy-page1-960px-Pike_Place_Market_sign_%28%22Public_Market_Center%22%29_with_clock%2C_Seattle%2C_Washington_LCCN2011630302.tif.jpg'),
    credit: { author: 'Carol M. Highsmith', license: 'Public domain', url: commons('Pike_Place_Market_sign_("Public_Market_Center")_with_clock,_Seattle,_Washington_LCCN2011630302.tif') },
  },
  {
    username: 'kerry_park_regular',
    hoursAgo: 100,
    message: 'Golden hour at Kerry Park. Worth the walk up Queen Anne hill every single time, rain or shine.',
    mediaUrl: thumb('e/e3/Seattle_Kerry_Park_Skyline.jpg/960px-Seattle_Kerry_Park_Skyline.jpg'),
    credit: { author: 'CommunistSquared', license: 'CC0', url: commons('Seattle_Kerry_Park_Skyline.jpg') },
  },
  {
    username: DEMO_USER,
    hoursAgo: 130,
    message: 'Seaplane coming in over Lake Union while I wait for the drizzle to pass. My first post!',
    mediaUrl: thumb('9/90/Lake_Union_-_Seaplane_and_Ship_%282873458435%29.jpg/960px-Lake_Union_-_Seaplane_and_Ship_%282873458435%29.jpg'),
    credit: { author: 'Daniel Ramirez', license: 'CC BY 2.0', url: commons('Lake_Union_-_Seaplane_and_Ship_(2873458435).jpg') },
  },
]

export function seedPosts(now = Date.now()): Post[] {
  return SEEDS.map(({ hoursAgo, ...post }, i) => ({
    ...post,
    id: `seed-${i + 1}`,
    mediaType: 'image',
    createdAt: new Date(now - hoursAgo * 3_600_000).toISOString(),
  }))
}
